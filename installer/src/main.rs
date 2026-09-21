mod config;
mod install;
mod openspec;
mod tool;
mod ui;

use anyhow::{Context, Result, bail};
use install::Installation;
use std::{collections::BTreeSet, env, io::IsTerminal, path::PathBuf};
use tool::Tool;

const HELP: &str = "North installer

Usage: ./install.sh [--tool opencode|claude] [--all | --skills NAME,NAME] [--openspec] [--merge]
       ./install.sh [--tool opencode|claude] --uninstall

With no options, choose a tool, then open its categorized installer checklist.
  --tool NAME    Install for opencode (${XDG_CONFIG_HOME:-$HOME/.config}/opencode)
                 or claude, meaning Claude Code (${CLAUDE_CONFIG_DIR:-$HOME/.claude})
                 Unattended options default to opencode
  --all          Enable all skills, Auto commit, and North pipeline
  --skills LIST  Select skills and workflows; include auto-commit for Auto commit
                 Otherwise commit is linked (use '' for only commit)
                 Include north-pipeline for its commands, skills, and memory
                 Required skill dependencies are included automatically
  --openspec     Install OpenSpec globally with npm if missing (Node.js 20.19.0+)
                 Alone, keeps the current skill selection (all on first install)
  --merge        OpenCode: merge North into opencode.json/jsonc; keep AGENTS.md
                 Claude Code: keep CLAUDE.md and add an @import of North's rules
                 Alone, keeps the current skill selection (all on first install)
  --uninstall    Remove North's links/config additions and restore saved instructions
  --help         Show this help

Interactive: Up/Down or j/k to move, Space to toggle, Enter to apply,
u to uninstall, Esc to choose another tool, q to quit without changes.";

fn main() {
    if let Err(error) = run() {
        eprintln!("North: {error:#}");
        std::process::exit(1);
    }
}

fn load(repo: &std::path::Path, tool: Tool) -> Result<Installation> {
    Installation::load(repo, tool, &tool.config_dir()?)
}

fn run() -> Result<()> {
    let mut args = env::args().skip(1);
    let mut repo = None;
    let mut tool = None;
    let mut action = None;
    let mut openspec = false;
    let mut merge = false;
    while let Some(arg) = args.next() {
        match arg.as_str() {
            "--help" | "-h" => {
                println!("{HELP}");
                return Ok(());
            }
            "--repo" if repo.is_none() => {
                repo = Some(PathBuf::from(args.next().context("--repo needs a path")?));
            }
            "--tool" if tool.is_none() => {
                let name = args.next().context("--tool needs opencode or claude")?;
                tool = Some(
                    Tool::parse(&name)
                        .with_context(|| format!("Unknown tool {name}; use opencode or claude"))?,
                );
            }
            "--openspec" if !openspec => openspec = true,
            "--merge" if !merge => merge = true,
            "--all" | "--skills" | "--uninstall" if action.is_none() => {
                action = Some(match arg.as_str() {
                    "--all" => ui::Action::Apply {
                        skills: None,
                        openspec: false,
                        merge: false,
                    },
                    "--uninstall" => ui::Action::Uninstall,
                    _ => {
                        let list = args
                            .next()
                            .context("--skills needs a comma-separated list")?;
                        let names = if list.is_empty() {
                            BTreeSet::new()
                        } else {
                            list.split(',').map(str::to_owned).collect()
                        };
                        ui::Action::Apply {
                            skills: Some(names),
                            openspec: false,
                            merge: false,
                        }
                    }
                });
            }
            _ => bail!("Unknown or conflicting option: {arg}\n{HELP}"),
        }
    }

    if (openspec || merge) && matches!(action, Some(ui::Action::Uninstall)) {
        bail!("--openspec and --merge cannot be combined with --uninstall");
    }

    let repo = repo.context("Run this installer through install.sh")?;
    let unattended = openspec || merge || action.is_some();
    let (installation, action) = if unattended {
        let installation = load(&repo, tool.unwrap_or_default())?;
        let action = match action {
            Some(ui::Action::Apply { skills, .. }) => ui::Action::Apply {
                skills,
                openspec,
                merge: merge || installation.merging(),
            },
            Some(ui::Action::Uninstall) => ui::Action::Uninstall,
            _ => ui::Action::Apply {
                skills: Some(installation.selected_skills()),
                openspec,
                merge: merge || installation.merging(),
            },
        };
        (installation, action)
    } else {
        if !std::io::stdin().is_terminal() || !std::io::stdout().is_terminal() {
            bail!(
                "The interactive installer needs a terminal. Use --all, --skills LIST, --openspec, --merge, or --uninstall for unattended use."
            );
        }
        let tools: Vec<Tool> = match tool {
            Some(tool) => vec![tool],
            None => Tool::ALL.to_vec(),
        };
        let mut candidates: Vec<_> = tools
            .into_iter()
            .map(|tool| ui::Candidate {
                tool,
                installation: load(&repo, tool).map_err(|error| format!("{error:#}")),
            })
            .collect();
        if let [single] = candidates.as_slice()
            && let Err(error) = &single.installation
        {
            bail!("{error}");
        }
        if candidates
            .iter()
            .all(|candidate| candidate.installation.is_err())
        {
            let problems: Vec<_> = candidates
                .iter()
                .filter_map(|candidate| {
                    candidate
                        .installation
                        .as_ref()
                        .err()
                        .map(|error| format!("{}: {error}", candidate.tool.label()))
                })
                .collect();
            bail!("No tool can be managed:\n{}", problems.join("\n"));
        }
        let (index, action) = ratatui::run(|terminal| ui::run(terminal, &candidates))?;
        let candidate = candidates.swap_remove(index);
        match candidate.installation {
            Ok(installation) => (installation, action),
            Err(_) => return finish_cancelled(),
        }
    };
    match action {
        ui::Action::Apply {
            skills,
            openspec,
            merge,
        } => {
            let selected = skills.unwrap_or_else(|| installation.skill_names());
            let resolved = installation.resolved_skills(&selected)?;
            installation.apply_with_merge(&selected, merge)?;
            println!(
                "North installed for {} in {} with {} enabled skills. Rerun ./install.sh to manage or uninstall it.",
                installation.tool.label(),
                installation.config.display(),
                resolved.len()
            );
            let included: Vec<_> = resolved.difference(&selected).cloned().collect();
            if !included.is_empty() {
                println!("Automatically included: {}.", included.join(", "));
            }
            if openspec {
                openspec::ensure_installed()
                    .context("North was saved, but OpenSpec setup failed")?;
            }
        }
        ui::Action::Uninstall => {
            installation.uninstall()?;
            println!(
                "North removed from {}, including its merged config additions. Any saved {} has been restored to {}.",
                installation.tool.label(),
                installation.tool.backup(),
                installation.tool.instructions()
            );
        }
        ui::Action::Cancel => return finish_cancelled(),
    }
    Ok(())
}

fn finish_cancelled() -> Result<()> {
    println!("No changes made.");
    Ok(())
}
