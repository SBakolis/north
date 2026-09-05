use crate::install::{AUTO_COMMIT, Installation};
use crossterm::event::{self, Event, KeyCode, KeyEventKind, KeyModifiers};
use ratatui::{
    DefaultTerminal, Frame,
    layout::{Alignment, Constraint, Layout},
    style::{Color, Modifier, Style},
    widgets::{Block, List, ListItem, ListState, Paragraph, Wrap},
};
use std::{collections::BTreeSet, io};

const NORTH_BANNER: &str = r" _   _  ___  ____ _____ _   _
| \ | |/ _ \|  _ \_   _| | | |
|  \| | | | | |_) || | | |_| |
| |\  | |_| |  _ < | | |  _  |
|_| \_|\___/|_| \_\|_| |_| |_|";

pub enum Action {
    Apply {
        skills: Option<BTreeSet<String>>,
        openspec: bool,
        merge: bool,
    },
    Uninstall,
    Cancel,
}

pub fn run(terminal: &mut DefaultTerminal, installation: &Installation) -> io::Result<Action> {
    let mut selected = installation.selected_skills();
    let mut openspec = false;
    let mut merge = installation.merging();
    let mut list = ListState::default().with_selected(Some(0));
    let mut confirming_uninstall = false;
    loop {
        terminal.draw(|frame| {
            render(
                frame,
                installation,
                &selected,
                &mut list,
                confirming_uninstall,
                openspec,
                merge,
            )
        })?;
        let Event::Key(key) = event::read()? else {
            continue;
        };
        if key.kind != KeyEventKind::Press {
            continue;
        }
        if key.code == KeyCode::Char('c') && key.modifiers.contains(KeyModifiers::CONTROL) {
            return Ok(Action::Cancel);
        }
        if confirming_uninstall {
            match key.code {
                KeyCode::Char('y') => return Ok(Action::Uninstall),
                KeyCode::Esc | KeyCode::Char('n') | KeyCode::Char('q') => {
                    confirming_uninstall = false
                }
                _ => {}
            }
            continue;
        }
        let count = installation.skills.len() + 2;
        match key.code {
            KeyCode::Esc | KeyCode::Char('q') => return Ok(Action::Cancel),
            KeyCode::Down | KeyCode::Char('j') if count > 0 => {
                list.select(Some((list.selected().unwrap_or(0) + 1) % count));
            }
            KeyCode::Up | KeyCode::Char('k') if count > 0 => {
                list.select(Some((list.selected().unwrap_or(0) + count - 1) % count));
            }
            KeyCode::Char(' ') if count > 0 => {
                if list.selected() == Some(installation.skills.len()) {
                    openspec = !openspec;
                    continue;
                }
                if list.selected() == Some(installation.skills.len() + 1) {
                    merge = !merge;
                    continue;
                }
                let name = &installation.skills[list.selected().unwrap_or(0)];
                if !selected.remove(name) {
                    selected.insert(name.clone());
                }
            }
            KeyCode::Char('a') => selected = installation.skill_names(),
            KeyCode::Char('n') => selected.clear(),
            KeyCode::Enter => {
                return Ok(Action::Apply {
                    skills: Some(selected),
                    openspec,
                    merge,
                });
            }
            KeyCode::Char('u') if installation.installed() => confirming_uninstall = true,
            _ => {}
        }
    }
}

fn render(
    frame: &mut Frame,
    installation: &Installation,
    selected: &BTreeSet<String>,
    list: &mut ListState,
    confirming: bool,
    openspec: bool,
    merge: bool,
) {
    // Keep both installation options visible in a standard 80x24 terminal.
    let full_banner = usize::from(frame.area().height) >= 19 + installation.skills.len()
        && usize::from(frame.area().width) >= NORTH_BANNER.lines().map(str::len).max().unwrap_or(0);
    let [banner, header, body, footer] = Layout::vertical([
        Constraint::Length(if full_banner { 5 } else { 1 }),
        Constraint::Length(5),
        Constraint::Min(3),
        Constraint::Length(5),
    ])
    .areas(frame.area());
    frame.render_widget(
        Paragraph::new(if full_banner { NORTH_BANNER } else { "NORTH" })
            .alignment(Alignment::Center)
            .style(
                Style::default()
                    .fg(Color::Cyan)
                    .add_modifier(Modifier::BOLD),
            ),
        banner,
    );
    let title = if installation.installed() {
        " North / Manage installation "
    } else {
        " North / Install "
    };
    let intro = format!(
        "{}\nChoose skills to enable. Shared instructions and North agents are included.\n{}",
        installation.config.display(),
        if merge {
            "Merge: keep AGENTS.md and combine existing OpenCode settings with North."
        } else {
            "Existing AGENTS.md is saved as AGENTS-backup.md on first installation."
        }
    );
    frame.render_widget(
        Paragraph::new(intro)
            .wrap(Wrap { trim: true })
            .block(Block::bordered().title(title)),
        header,
    );
    let mut items: Vec<_> = installation
        .skills
        .iter()
        .map(|name| {
            let label = if name == AUTO_COMMIT {
                "Auto commit"
            } else {
                name
            };
            ListItem::new(format!(
                "[{}] {label}",
                if selected.contains(name) { "x" } else { " " }
            ))
        })
        .collect();
    items.push(ListItem::new(format!(
        "[{}] OpenSpec CLI (install if missing; npm global)",
        if openspec { "x" } else { " " }
    )));
    items.push(ListItem::new(format!(
        "[{}] Merge installations (opencode.json / opencode.jsonc)",
        if merge { "x" } else { " " }
    )));
    frame.render_stateful_widget(
        List::new(items)
            .block(Block::bordered().title(format!(
                " Skills / {} enabled + installation options ",
                installation.resolved_skills(selected).map_or(selected.len(), |skills| skills.len())
            )))
            .highlight_style(
                Style::default()
                    .fg(Color::Cyan)
                    .add_modifier(Modifier::BOLD),
            )
            .highlight_symbol("> "),
        body,
        list,
    );
    let help = if confirming {
        "Uninstall North and undo its instructions/config additions?\nPress y to uninstall; n or Esc to return. Your settings are preserved."
    } else if installation.installed() {
        "Up/Down or j/k: move   Space: toggle   a/n: all/no skill options\nEnter: save changes   u: uninstall North   q/Esc: quit without changes\nAuto commit: on commits automatically; off waits for your go-ahead."
    } else {
        "Up/Down or j/k: move   Space: toggle   a/n: all/no skill options\nEnter: install North   q/Esc: quit without changes\nAuto commit: on commits automatically; off waits for your go-ahead."
    };
    frame.render_widget(
        Paragraph::new(help)
            .wrap(Wrap { trim: true })
            .style(Style::default().fg(if confirming {
                Color::Yellow
            } else {
                Color::Gray
            }))
            .block(Block::bordered().title(if confirming {
                " Confirm uninstall "
            } else {
                " Controls "
            })),
        footer,
    );
}

#[cfg(test)]
mod tests {
    use super::*;
    use ratatui::{Terminal, backend::TestBackend};

    #[test]
    fn checklist_and_uninstall_confirmation_render_in_small_terminals() {
        let repo = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .parent()
            .unwrap();
        let temp = tempfile::tempdir().unwrap();
        let installation = Installation::load(repo, &temp.path().join("config")).unwrap();
        for (width, height) in [(80, 24), (35, 12), (10, 5)] {
            let mut terminal = Terminal::new(TestBackend::new(width, height)).unwrap();
            for confirming in [false, true] {
                terminal
                    .draw(|frame| {
                        render(
                            frame,
                            &installation,
                            &installation.selected_skills(),
                            &mut ListState::default().with_selected(Some(0)),
                            confirming,
                            true,
                            true,
                        )
                    })
                    .unwrap();
                if width == 80 {
                    let text: String = terminal
                        .backend()
                        .buffer()
                        .content
                        .iter()
                        .map(|cell| cell.symbol())
                        .collect();
                    assert!(text.contains("[x] explain-code"));
                    assert!(text.contains("[x] Auto commit"));
                    assert!(!text.contains("[x] commit"));
                    assert!(!text.contains("[x] auto-commit"));
                    assert!(text.contains("[x] OpenSpec CLI (install if missing; npm global)"));
                    assert!(
                        text.contains("[x] Merge installations (opencode.json / opencode.jsonc)")
                    );
                    assert!(text.contains(if confirming {
                        "Confirm uninstall"
                    } else {
                        "Enter: install North"
                    }));
                }
            }
        }
    }

    #[test]
    fn unchecked_auto_commit_still_counts_the_manual_skill() {
        let repo = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .parent()
            .unwrap();
        let temp = tempfile::tempdir().unwrap();
        let installation = Installation::load(repo, &temp.path().join("config")).unwrap();
        let mut terminal = Terminal::new(TestBackend::new(80, 24)).unwrap();
        terminal
            .draw(|frame| {
                render(
                    frame,
                    &installation,
                    &BTreeSet::new(),
                    &mut ListState::default().with_selected(Some(0)),
                    false,
                    false,
                    false,
                )
            })
            .unwrap();
        let text: String = terminal
            .backend()
            .buffer()
            .content
            .iter()
            .map(|cell| cell.symbol())
            .collect();
        assert_eq!(text.matches("[ ] Auto commit").count(), 1);
        assert!(!text.contains("[ ] commit"));
        assert!(text.contains("Skills / 1 enabled"));
    }
}
