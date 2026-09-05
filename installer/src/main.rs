mod install;
mod ui;

use anyhow::{Context, Result, bail};
use install::Installation;
use std::{collections::BTreeSet, env, io::IsTerminal, path::PathBuf};

const HELP: &str = "North installer

Usage: ./install.sh [--all | --skills NAME,NAME | --uninstall]

With no options, open the interactive skill checklist.
  --all          Install North with all bundled skills
  --skills LIST  Install/update with exactly these skills (use '' for none)
  --uninstall    Remove North and restore AGENTS-backup.md
  --help         Show this help

Interactive: Up/Down or j/k to move, Space to toggle, Enter to apply,
u to uninstall, Esc/q to quit without changes.";

fn main() {
    if let Err(error) = run() {
        eprintln!("North: {error:#}");
        std::process::exit(1);
    }
}

fn run() -> Result<()> {
    let mut args = env::args().skip(1);
    let mut repo = None;
    let mut action = None;
    while let Some(arg) = args.next() {
        match arg.as_str() {
            "--help" | "-h" => {
                println!("{HELP}");
                return Ok(());
            }
            "--repo" if repo.is_none() => {
                repo = Some(PathBuf::from(args.next().context("--repo needs a path")?));
            }
            "--all" | "--skills" | "--uninstall" if action.is_none() => {
                action = Some(match arg.as_str() {
                    "--all" => ui::Action::Apply(None),
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
                        ui::Action::Apply(Some(names))
                    }
                });
            }
            _ => bail!("Unknown or conflicting option: {arg}\n{HELP}"),
        }
    }

    let repo = repo.context("Run this installer through install.sh")?;
    let base = env::var_os("XDG_CONFIG_HOME")
        .filter(|value| !value.is_empty())
        .map(PathBuf::from)
        .or_else(|| env::var_os("HOME").map(|home| PathBuf::from(home).join(".config")))
        .context("Set HOME or XDG_CONFIG_HOME")?;
    if !base.is_absolute() {
        bail!("XDG_CONFIG_HOME (or HOME) must be an absolute path");
    }
    let installation = Installation::load(&repo, &base.join("opencode"))?;
    let action = match action {
        Some(action) => action,
        None => {
            if !std::io::stdin().is_terminal() || !std::io::stdout().is_terminal() {
                bail!(
                    "The interactive installer needs a terminal. Use --all, --skills LIST, or --uninstall for unattended use."
                );
            }
            ratatui::run(|terminal| ui::run(terminal, &installation))?
        }
    };
    match action {
        ui::Action::Apply(selected) => {
            let selected = selected.unwrap_or_else(|| installation.skill_names());
            installation.apply(&selected)?;
            println!(
                "North installed in {} with {} enabled skills. Rerun ./install.sh to manage or uninstall it.",
                installation.config.display(),
                selected.len()
            );
        }
        ui::Action::Uninstall => {
            installation.uninstall()?;
            println!("North removed. Any saved AGENTS-backup.md has been restored to AGENTS.md.");
        }
        ui::Action::Cancel => println!("No changes made."),
    }
    Ok(())
}
