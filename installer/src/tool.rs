use anyhow::{Context, Result, bail};
use serde::{Deserialize, Serialize};
use std::{env, path::PathBuf};

/// A supported coding agent whose global configuration receives North's links.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Tool {
    #[default]
    OpenCode,
    Claude,
}

impl Tool {
    pub const ALL: [Tool; 2] = [Tool::OpenCode, Tool::Claude];

    pub fn parse(name: &str) -> Option<Tool> {
        match name {
            "opencode" => Some(Tool::OpenCode),
            "claude" | "claude-code" => Some(Tool::Claude),
            _ => None,
        }
    }

    pub fn id(self) -> &'static str {
        match self {
            Tool::OpenCode => "opencode",
            Tool::Claude => "claude",
        }
    }

    pub fn label(self) -> &'static str {
        match self {
            Tool::OpenCode => "OpenCode",
            Tool::Claude => "Claude Code",
        }
    }

    /// Global instructions file name inside the configuration directory.
    pub fn instructions(self) -> &'static str {
        match self {
            Tool::OpenCode => "AGENTS.md",
            Tool::Claude => "CLAUDE.md",
        }
    }

    /// Name used to preserve the user's original instructions.
    pub fn backup(self) -> &'static str {
        match self {
            Tool::OpenCode => "AGENTS-backup.md",
            Tool::Claude => "CLAUDE-backup.md",
        }
    }

    /// Files that merge mode edits instead of replacing the instructions file.
    pub fn config_files(self) -> &'static [&'static str] {
        match self {
            Tool::OpenCode => &["opencode.json", "opencode.jsonc"],
            Tool::Claude => &["CLAUDE.md"],
        }
    }

    /// Commands owned by the North pipeline. Claude Code exposes the pipeline
    /// skills as slash commands directly, so it needs no command wrappers.
    pub fn pipeline_commands(self) -> &'static [&'static str] {
        match self {
            Tool::OpenCode => &["north-plan.md", "north-execute.md", "north-save.md"],
            Tool::Claude => &[],
        }
    }

    pub fn merge_label(self) -> &'static str {
        match self {
            Tool::OpenCode => "Merge installations (opencode.json / opencode.jsonc)",
            Tool::Claude => "Merge installations (import North into CLAUDE.md)",
        }
    }

    pub fn merge_summary(self) -> &'static str {
        match self {
            Tool::OpenCode => {
                "Merge: keep AGENTS.md and combine existing OpenCode settings with North."
            }
            Tool::Claude => "Merge: keep CLAUDE.md and add an @import of North's instructions.",
        }
    }

    pub fn replace_summary(self) -> &'static str {
        match self {
            Tool::OpenCode => {
                "Existing AGENTS.md is saved as AGENTS-backup.md on first installation."
            }
            Tool::Claude => {
                "Existing CLAUDE.md is saved as CLAUDE-backup.md on first installation."
            }
        }
    }

    fn env_dir(name: &str) -> Option<PathBuf> {
        env::var_os(name)
            .filter(|value| !value.is_empty())
            .map(PathBuf::from)
    }

    /// Resolve the tool's global configuration directory from the environment.
    pub fn config_dir(self) -> Result<PathBuf> {
        let home = || Self::env_dir("HOME");
        let (dir, variables) = match self {
            Tool::OpenCode => (
                Self::env_dir("XDG_CONFIG_HOME")
                    .or_else(|| home().map(|home| home.join(".config")))
                    .map(|base| base.join("opencode")),
                "XDG_CONFIG_HOME (or HOME)",
            ),
            Tool::Claude => (
                Self::env_dir("CLAUDE_CONFIG_DIR")
                    .or_else(|| home().map(|home| home.join(".claude"))),
                "CLAUDE_CONFIG_DIR (or HOME)",
            ),
        };
        let dir = dir.with_context(|| format!("Set {variables}"))?;
        if !dir.is_absolute() {
            bail!("{variables} must be an absolute path");
        }
        Ok(dir)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn names_round_trip_and_reject_unknown_tools() {
        for tool in Tool::ALL {
            assert_eq!(Tool::parse(tool.id()), Some(tool));
            assert_eq!(
                serde_json::from_str::<Tool>(&serde_json::to_string(&tool).unwrap()).unwrap(),
                tool
            );
        }
        assert_eq!(Tool::parse("claude-code"), Some(Tool::Claude));
        assert_eq!(Tool::parse("cursor"), None);
        assert_eq!(serde_json::to_string(&Tool::Claude).unwrap(), "\"claude\"");
    }

    #[test]
    fn tools_use_distinct_files_and_pipeline_commands() {
        assert_ne!(Tool::OpenCode.instructions(), Tool::Claude.instructions());
        assert_ne!(Tool::OpenCode.backup(), Tool::Claude.backup());
        assert!(Tool::Claude.pipeline_commands().is_empty());
        assert_eq!(Tool::OpenCode.pipeline_commands().len(), 3);
        assert_eq!(Tool::Claude.config_files(), &["CLAUDE.md"]);
    }
}
