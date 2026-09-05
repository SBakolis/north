import path from "node:path"

const gitCommand = /(^|[;&|()\s])(?:[^\s;&|()]*\/)?git\b[^;&|\n]*/g
const forbiddenGitOperation = /(^|\s)(merge|rebase|push|worktree|clean|update-ref|symbolic-ref)(\s|$)|(^|\s)(branch|tag)\b[^;&|\n]*(^|\s)(-[dD]|--delete)(\s|$)|(^|\s)reset\b[^;&|\n]*(^|\s)--hard(\s|$)|(^|\s)-c\s+alias\.|(^|\s)--exec-path/
const nestedNorth = /(^|\s)((?:[^\s;&|]+\/)?north\s+run|\/loop)(\s|$)/
const protectedEnvironmentPath = /\$(?:\{)?(?:NORTH_STATE_DIR|HOME|XDG_CONFIG_HOME|XDG_DATA_HOME|XDG_STATE_HOME|XDG_CACHE_HOME)(?:\})?/
const worktreeParentPath = /\$(?:\{)?NORTH_WORKTREE(?:\})?\/\.\.(?:\/|\s|[;&|]|$)/
const dynamicShellCommand = /(^|[;&|()]\s*)(?:[A-Za-z_][A-Za-z0-9_]*=[^\s;&|]+\s+)*["']?\$(?:\{|[A-Za-z_])|(^|\s)eval(?:\s|$)/
const forbiddenGitWords = /\b(?:merge|rebase|push|worktree|clean|update-ref|symbolic-ref|branch|tag|reset)\b/

function hasDynamicCommandWord(command) {
  return command.split(/[;&|()]/).some((segment) => {
    const commandWord = segment.trim().replace(/^(?:[A-Za-z_][A-Za-z0-9_]*=\S+\s+)*/, "").match(/^\S+/)?.[0] ?? ""
    return /[$`]/.test(commandWord)
  })
}

function inside(root, candidate) {
  const relative = path.relative(path.resolve(root), path.resolve(candidate))
  return relative === "" || (!relative.startsWith(`..${path.sep}`) && relative !== ".." && !path.isAbsolute(relative))
}

function invokesForbiddenGit(command) {
  // Shells remove quotes and backslashes before command lookup. Normalize those
  // forms so g""it, g\it, and p\ush cannot disguise host-owned operations.
  const dequoted = command
    .replace(/\\([^\n])/g, "$1")
    .replace(/["']/g, "")
  const normalized = dequoted
    .replace(/\$\{[^}]*\}|\$[A-Za-z_][A-Za-z0-9_]*/g, "")
    .replace(/\$\([^)]*\)|`[^`]*`/g, "")
  if (/\$\(|`/.test(command) || dynamicShellCommand.test(command) || hasDynamicCommandWord(command)) return true
  if (/\$(?:\{[^}]*\}|[A-Za-z_][A-Za-z0-9_]*).*\b(?:merge|rebase|push|worktree|clean|update-ref|symbolic-ref|branch|tag|reset)\b/.test(dequoted)) return true
  if (/\b(?:sh|bash|zsh)\s+-c\s+["']?\$/.test(dequoted)) return true
  if (/\bgit\s+/.test(dequoted) && forbiddenGitWords.test(dequoted)) return true
  if ([...command.matchAll(gitCommand)].some((match) => /\$(?:\{|[A-Za-z_(])|`/.test(match[0]))) return true
  if (/(?:\$\{[^}]*\}|\$[A-Za-z_][A-Za-z0-9_]*)\s+(?:merge|rebase|push|worktree|clean|update-ref|symbolic-ref|branch|tag|reset)(?:\s|$)/.test(dequoted)) return true
  return [...normalized.matchAll(gitCommand)].some((match) => forbiddenGitOperation.test(match[0]))
}

function commandEscapesWorktree(command, worktree) {
  if (protectedEnvironmentPath.test(command) || worktreeParentPath.test(command) || /\$\{NORTH_WORKTREE[^}]/.test(command)) return true
  const paths = command.match(/(?:^|[\s"'=])((?:\.\.(?:\/[^\s"';&|]*)?|\/[^\s"';&|]*))/g) ?? []
  return paths.some((match) => {
    const candidate = match.trim().replace(/^["'=]+|["']+$/g, "")
    return candidate !== "/dev/null" && !inside(worktree, path.resolve(worktree, candidate))
  })
}

/** @type {import("@opencode-ai/plugin").Plugin} */
export const NorthGuardrails = async () => {
  if (process.env.NORTH_ACTIVE !== "1") return {}

  const worktree = process.env.NORTH_WORKTREE
  const stateDir = process.env.NORTH_STATE_DIR
  if (!worktree || !stateDir) throw new Error("North guardrails require NORTH_WORKTREE and NORTH_STATE_DIR")

  return {
    "tool.execute.before": async (input, output) => {
      const args = output.args
      const candidate = ["filePath", "path", "directory"].map((key) => args[key]).find((value) => typeof value === "string")
      if (typeof candidate === "string") {
        const absolute = path.isAbsolute(candidate) ? candidate : path.join(worktree, candidate)
        if (!inside(worktree, absolute) || inside(stateDir, absolute)) {
          throw new Error(`North denied ${input.tool} outside the assigned worktree`)
        }
      }

      if (input.tool === "bash" && typeof args.command === "string") {
        if (commandEscapesWorktree(args.command, worktree)) throw new Error("North denied a shell path outside the assigned worktree")
        if (invokesForbiddenGit(args.command)) throw new Error("North host owns Git integration and worktrees")
        if (nestedNorth.test(args.command)) throw new Error("Nested North and /loop execution is prohibited")
      }
    },
  }
}
