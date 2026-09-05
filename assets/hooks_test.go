package assets

import (
	"encoding/base64"
	"os/exec"
	"testing"
)

func TestGuardrailRejectsGitGlobalOptionsAndLongDelete(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is unavailable")
	}
	source, err := FS.ReadFile("hooks/north-guardrails.ts")
	if err != nil {
		t.Fatal(err)
	}
	script := `
const module = await import("data:text/javascript;base64," + process.argv[1]);
process.env.NORTH_ACTIVE = "1";
process.env.NORTH_WORKTREE = "/tmp/north-worktree";
process.env.NORTH_STATE_DIR = "/tmp/north-state";
const hooks = await module.NorthGuardrails();
	for (const command of ["git -C . push", "git branch --delete topic", "git update-ref -d refs/heads/main", "/usr/bin/git reset -q --hard HEAD", "git -c alias.p=push p", "g\"\"it push", "g\\it push", "git p\\ush", "g${EMPTY}it push", "I=i; g${I}t push", "$(printf git) push", "$GIT push", "G=git; \"$G\" push", "operation=push; git $operation", "CMD='git push'; sh -c \"$CMD\"", "eval $COMMAND", "printf \"\\147\\151\\164\\040\\160\\165\\163\\150\" | sh", "python3 -c '__import__(\"os\").system(__import__(\"base64\").b64decode(\"Z2l0IHB1c2g=\").decode())'"]) {
  let denied = false;
  try { await hooks["tool.execute.before"]({tool: "bash"}, {args: {command}}); } catch { denied = true; }
  if (!denied) throw new Error("allowed forbidden command: " + command);
}
	for (const command of ["rm $NORTH_STATE_DIR/run.json", "rm ../outside", "cat ./../outside", "cat subdir/../../outside", "cd ..; rm victim", "cd $NORTH_WORKTREE/..; touch victim", "rm \"${NORTH_WORKTREE%/*}/victim\"", "/usr/local/bin/north run plan.yaml", "git -C/tmp diff --stat", "cc -o/tmp/output source.c", "python3 -c'__import__(\"os\").system(\"noop\")'", "node -eprocess.mainModule.require(\"child_process\").execFileSync(\"g\"+\"it\",[\"p\"+\"ush\"])"]) {
  let denied = false;
  try { await hooks["tool.execute.before"]({tool: "bash"}, {args: {command}}); } catch { denied = true; }
  if (!denied) throw new Error("allowed boundary escape: " + command);
}
await hooks["tool.execute.before"]({tool: "bash"}, {args: {command: "git -C . diff --stat"}});
await hooks["tool.execute.before"]({tool: "bash"}, {args: {command: "go test ./..."}});
`
	command := exec.Command("node", "--input-type=module", "-e", script, base64.StdEncoding.EncodeToString(source))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("guardrail test: %v: %s", err, output)
	}
}
