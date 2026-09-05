use crate::install::Installation;
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
    Apply(Option<BTreeSet<String>>),
    Uninstall,
    Cancel,
}

pub fn run(terminal: &mut DefaultTerminal, installation: &Installation) -> io::Result<Action> {
    let mut selected = installation.selected_skills();
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
        let count = installation.skills.len();
        match key.code {
            KeyCode::Esc | KeyCode::Char('q') => return Ok(Action::Cancel),
            KeyCode::Down | KeyCode::Char('j') if count > 0 => {
                list.select(Some((list.selected().unwrap_or(0) + 1) % count));
            }
            KeyCode::Up | KeyCode::Char('k') if count > 0 => {
                list.select(Some((list.selected().unwrap_or(0) + count - 1) % count));
            }
            KeyCode::Char(' ') if count > 0 => {
                let name = &installation.skills[list.selected().unwrap_or(0)];
                if !selected.remove(name) {
                    selected.insert(name.clone());
                }
            }
            KeyCode::Char('a') => selected = installation.skill_names(),
            KeyCode::Char('n') => selected.clear(),
            KeyCode::Enter => return Ok(Action::Apply(Some(selected))),
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
) {
    let full_banner = frame.area().height >= 20
        && usize::from(frame.area().width) >= NORTH_BANNER.lines().map(str::len).max().unwrap_or(0);
    let [banner, header, body, footer] = Layout::vertical([
        Constraint::Length(if full_banner { 6 } else { 1 }),
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
        "{}\nChoose skills to enable. Shared instructions and North agents are included.\nExisting AGENTS.md is saved as AGENTS-backup.md on first installation.",
        installation.config.display()
    );
    frame.render_widget(
        Paragraph::new(intro)
            .wrap(Wrap { trim: true })
            .block(Block::bordered().title(title)),
        header,
    );
    let items: Vec<_> = installation
        .skills
        .iter()
        .map(|name| {
            ListItem::new(format!(
                "[{}] {name}",
                if selected.contains(name) { "x" } else { " " }
            ))
        })
        .collect();
    frame.render_stateful_widget(
        List::new(items)
            .block(Block::bordered().title(format!(" Skills / {} enabled ", selected.len())))
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
        "Uninstall North and restore your original AGENTS.md?\nPress y to uninstall; n or Esc to return. Other OpenCode files are preserved."
    } else if installation.installed() {
        "Up/Down or j/k: move   Space: toggle   a: all   n: none\nEnter: save changes   u: uninstall North   q/Esc: quit without changes"
    } else {
        "Up/Down or j/k: move   Space: toggle   a: all   n: none\nEnter: install North   q/Esc: quit without changes"
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
                    assert!(text.contains(if confirming {
                        "Confirm uninstall"
                    } else {
                        "Enter: install North"
                    }));
                }
            }
        }
    }
}
