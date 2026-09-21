use crate::install::{
    AUTO_COMMIT, Installation, NORTH_PIPELINE, PIPELINE_SKILLS, pipeline_enabled,
};
use crate::tool::Tool;
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

/// One installable tool and its loaded installation, or why it cannot be managed.
pub struct Candidate {
    pub tool: Tool,
    pub installation: Result<Installation, String>,
}

impl Candidate {
    fn status(&self) -> String {
        match &self.installation {
            Ok(installation) if installation.installed() => {
                format!("installed in {}", installation.config.display())
            }
            Ok(installation) => format!("not installed; uses {}", installation.config.display()),
            Err(error) => format!("unavailable: {error}"),
        }
    }
}

#[derive(Debug, PartialEq, Eq)]
enum Entry {
    Heading(&'static str),
    AutoCommit,
    Pipeline,
    Skill(String),
    OpenSpec,
    Merge,
}

impl Entry {
    fn label(&self, tool: Tool) -> &str {
        match self {
            Self::Heading(label) => label,
            Self::Skill(label) => label,
            Self::AutoCommit => "Auto commit",
            Self::Pipeline => "North pipeline",
            Self::OpenSpec => "OpenSpec CLI (install if missing; npm global)",
            Self::Merge => tool.merge_label(),
        }
    }

    fn selectable(&self) -> bool {
        !matches!(self, Self::Heading(_))
    }
}

fn regular_skills(installation: &Installation) -> BTreeSet<String> {
    installation
        .skill_names()
        .into_iter()
        .filter(|name| name != AUTO_COMMIT && name != NORTH_PIPELINE)
        .collect()
}

fn entries(installation: &Installation) -> Vec<Entry> {
    let options = installation.skill_names();
    let mut entries = vec![Entry::Heading("Workflow")];
    if options.contains(AUTO_COMMIT) {
        entries.push(Entry::AutoCommit);
    }
    if options.contains(NORTH_PIPELINE) {
        entries.push(Entry::Pipeline);
    }
    entries.push(Entry::Heading("Skills"));
    entries.extend(regular_skills(installation).into_iter().map(Entry::Skill));
    entries.extend([
        Entry::Heading("OpenSpec"),
        Entry::OpenSpec,
        Entry::Heading("Installation"),
        Entry::Merge,
    ]);
    entries
}

fn move_selection(entries: &[Entry], list: &mut ListState, forward: bool) {
    let count = entries.len();
    for offset in 1..=count {
        let index = if forward {
            (list.selected().unwrap_or(count - 1) + offset) % count
        } else {
            (list.selected().unwrap_or(0) + count - offset) % count
        };
        if entries[index].selectable() {
            list.select(Some(index));
            return;
        }
    }
}

fn toggle_entry(
    entry: &Entry,
    selected: &mut BTreeSet<String>,
    openspec: &mut bool,
    merge: &mut bool,
) {
    let name = match entry {
        Entry::AutoCommit => AUTO_COMMIT,
        Entry::Pipeline => {
            let was_enabled = pipeline_enabled(selected);
            selected.remove(NORTH_PIPELINE);
            for name in PIPELINE_SKILLS {
                selected.remove(*name);
            }
            if !was_enabled {
                selected.insert(NORTH_PIPELINE.into());
            }
            return;
        }
        Entry::Skill(name) => name,
        Entry::OpenSpec => {
            *openspec = !*openspec;
            return;
        }
        Entry::Merge => {
            *merge = !*merge;
            return;
        }
        Entry::Heading(_) => return,
    };
    if !selected.remove(name) {
        selected.insert(name.into());
    }
}

fn select_regular_skills(
    installation: &Installation,
    selected: &mut BTreeSet<String>,
    enabled: bool,
) {
    let regular = regular_skills(installation);
    if enabled {
        selected.extend(regular);
    } else {
        selected.retain(|name| !regular.contains(name));
    }
}

/// Ask which tool to install for, then manage that tool's installation.
/// With a single candidate the tool question is skipped.
pub fn run(
    terminal: &mut DefaultTerminal,
    candidates: &[Candidate],
) -> io::Result<(usize, Action)> {
    let choose = candidates.len() > 1;
    let mut index = candidates
        .iter()
        .position(|candidate| candidate.installation.is_ok())
        .unwrap_or(0);
    loop {
        if choose {
            match choose_tool(terminal, candidates, index)? {
                Some(chosen) => index = chosen,
                None => return Ok((index, Action::Cancel)),
            }
        }
        let installation = candidates[index]
            .installation
            .as_ref()
            .expect("only loadable installations are chosen");
        match checklist(terminal, installation, choose)? {
            Some(action) => return Ok((index, action)),
            None => continue,
        }
    }
}

fn choose_tool(
    terminal: &mut DefaultTerminal,
    candidates: &[Candidate],
    initial: usize,
) -> io::Result<Option<usize>> {
    let mut list = ListState::default().with_selected(Some(initial));
    loop {
        terminal.draw(|frame| render_tools(frame, candidates, &mut list))?;
        let Event::Key(key) = event::read()? else {
            continue;
        };
        if key.kind != KeyEventKind::Press {
            continue;
        }
        if key.code == KeyCode::Char('c') && key.modifiers.contains(KeyModifiers::CONTROL) {
            return Ok(None);
        }
        let current = list.selected().unwrap_or(0);
        match key.code {
            KeyCode::Esc | KeyCode::Char('q') => return Ok(None),
            KeyCode::Down | KeyCode::Char('j') => {
                list.select(Some((current + 1) % candidates.len()));
            }
            KeyCode::Up | KeyCode::Char('k') => {
                list.select(Some((current + candidates.len() - 1) % candidates.len()));
            }
            KeyCode::Enter | KeyCode::Char(' ') if candidates[current].installation.is_ok() => {
                return Ok(Some(current));
            }
            _ => {}
        }
    }
}

fn render_tools(frame: &mut Frame, candidates: &[Candidate], list: &mut ListState) {
    let [banner, header, body, footer] = layout(frame, candidates.len());
    render_banner(frame, banner);
    frame.render_widget(
        Paragraph::new(
            "Choose the tool to install North for. Each tool keeps its own installation, links, and skill selection.",
        )
        .wrap(Wrap { trim: true })
        .block(Block::bordered().title(" North / Choose tool ")),
        header,
    );
    let items: Vec<_> = candidates
        .iter()
        .map(|candidate| {
            let item = ListItem::new(format!(
                "{:<12}{}",
                candidate.tool.label(),
                candidate.status()
            ));
            if candidate.installation.is_err() {
                item.style(Style::default().fg(Color::DarkGray))
            } else {
                item
            }
        })
        .collect();
    frame.render_stateful_widget(
        List::new(items)
            .block(Block::bordered().title(" Tools "))
            .highlight_style(
                Style::default()
                    .fg(Color::Cyan)
                    .add_modifier(Modifier::BOLD),
            )
            .highlight_symbol("> "),
        body,
        list,
    );
    let blocked = list
        .selected()
        .and_then(|index| candidates.get(index))
        .and_then(|candidate| candidate.installation.as_ref().err());
    let help = match blocked {
        Some(error) => format!(
            "This installation cannot be managed until the problem is fixed:\n{error}\nUp/Down or j/k: move   q/Esc: quit"
        ),
        None => "Up/Down or j/k: move   Enter: continue with the selected tool\nq/Esc: quit without changes".into(),
    };
    frame.render_widget(
        Paragraph::new(help)
            .wrap(Wrap { trim: true })
            .style(Style::default().fg(if blocked.is_some() {
                Color::Yellow
            } else {
                Color::Gray
            }))
            .block(Block::bordered().title(" Controls ")),
        footer,
    );
}

fn checklist(
    terminal: &mut DefaultTerminal,
    installation: &Installation,
    back: bool,
) -> io::Result<Option<Action>> {
    let mut selected = installation.selected_skills();
    let mut openspec = false;
    let mut merge = installation.merging();
    let entries = entries(installation);
    let mut list = ListState::default();
    move_selection(&entries, &mut list, true);
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
                back,
            )
        })?;
        let Event::Key(key) = event::read()? else {
            continue;
        };
        if key.kind != KeyEventKind::Press {
            continue;
        }
        if key.code == KeyCode::Char('c') && key.modifiers.contains(KeyModifiers::CONTROL) {
            return Ok(Some(Action::Cancel));
        }
        if confirming_uninstall {
            match key.code {
                KeyCode::Char('y') => return Ok(Some(Action::Uninstall)),
                KeyCode::Esc | KeyCode::Char('n') | KeyCode::Char('q') => {
                    confirming_uninstall = false
                }
                _ => {}
            }
            continue;
        }
        match key.code {
            KeyCode::Esc | KeyCode::Backspace if back => return Ok(None),
            KeyCode::Esc | KeyCode::Char('q') => return Ok(Some(Action::Cancel)),
            KeyCode::Down | KeyCode::Char('j') => {
                move_selection(&entries, &mut list, true);
            }
            KeyCode::Up | KeyCode::Char('k') => {
                move_selection(&entries, &mut list, false);
            }
            KeyCode::Char(' ') => {
                if let Some(entry) = list.selected().and_then(|index| entries.get(index)) {
                    toggle_entry(entry, &mut selected, &mut openspec, &mut merge);
                }
            }
            KeyCode::Char('a') => select_regular_skills(installation, &mut selected, true),
            KeyCode::Char('n') => select_regular_skills(installation, &mut selected, false),
            KeyCode::Enter => {
                return Ok(Some(Action::Apply {
                    skills: Some(selected),
                    openspec,
                    merge,
                }));
            }
            KeyCode::Char('u') if installation.installed() => confirming_uninstall = true,
            _ => {}
        }
    }
}

// Leave more room for the scrolling list when the catalog is long.
fn layout(frame: &Frame, rows: usize) -> [ratatui::layout::Rect; 4] {
    let full_banner = usize::from(frame.area().height) >= 17 + rows
        && usize::from(frame.area().width) >= NORTH_BANNER.lines().map(str::len).max().unwrap_or(0);
    Layout::vertical([
        Constraint::Length(if full_banner { 5 } else { 1 }),
        Constraint::Length(5),
        Constraint::Min(3),
        Constraint::Length(5),
    ])
    .areas(frame.area())
}

fn render_banner(frame: &mut Frame, area: ratatui::layout::Rect) {
    frame.render_widget(
        Paragraph::new(if area.height >= 5 {
            NORTH_BANNER
        } else {
            "NORTH"
        })
        .alignment(Alignment::Center)
        .style(
            Style::default()
                .fg(Color::Cyan)
                .add_modifier(Modifier::BOLD),
        ),
        area,
    );
}

#[allow(clippy::too_many_arguments)]
fn render(
    frame: &mut Frame,
    installation: &Installation,
    selected: &BTreeSet<String>,
    list: &mut ListState,
    confirming: bool,
    openspec: bool,
    merge: bool,
    back: bool,
) {
    let tool = installation.tool;
    let entries = entries(installation);
    let [banner, header, body, footer] = layout(frame, entries.len());
    render_banner(frame, banner);
    let title = if installation.installed() {
        format!(" North / {} / Manage installation ", tool.label())
    } else {
        format!(" North / {} / Install ", tool.label())
    };
    let intro = format!(
        "{}\nChoose workflows and skills. Instructions, agents and /north are included.\n{}",
        installation.config.display(),
        if merge {
            tool.merge_summary()
        } else {
            tool.replace_summary()
        }
    );
    frame.render_widget(
        Paragraph::new(intro)
            .wrap(Wrap { trim: true })
            .block(Block::bordered().title(title)),
        header,
    );
    let resolved = installation
        .resolved_skills(selected)
        .unwrap_or_else(|_| selected.clone());
    let items: Vec<_> = entries
        .iter()
        .map(|entry| {
            if let Entry::Heading(label) = entry {
                return ListItem::new(*label).style(
                    Style::default()
                        .fg(Color::Cyan)
                        .add_modifier(Modifier::BOLD),
                );
            }
            let (enabled, required) = match entry {
                Entry::AutoCommit => (selected.contains(AUTO_COMMIT), false),
                Entry::Pipeline => (pipeline_enabled(selected), false),
                Entry::Skill(name) => (
                    selected.contains(name),
                    !selected.contains(name) && resolved.contains(name),
                ),
                Entry::OpenSpec => (openspec, false),
                Entry::Merge => (merge, false),
                Entry::Heading(_) => unreachable!(),
            };
            ListItem::new(format!(
                "[{}] {}{}",
                if enabled {
                    "x"
                } else if required {
                    "+"
                } else {
                    " "
                },
                entry.label(tool),
                if required { " (required)" } else { "" }
            ))
        })
        .collect();
    frame.render_stateful_widget(
        List::new(items)
            .block(
                Block::bordered().title(format!(" Options / {} skills enabled ", resolved.len())),
            )
            .highlight_style(
                Style::default()
                    .fg(Color::Cyan)
                    .add_modifier(Modifier::BOLD),
            )
            .highlight_symbol("> "),
        body,
        list,
    );
    let leave = if back {
        "Esc: choose another tool   q: quit without changes"
    } else {
        "q/Esc: quit without changes"
    };
    let help = if confirming {
        "Uninstall North and undo its instructions/config additions?\nPress y to uninstall; n or Esc to return. Your settings are preserved.".to_owned()
    } else if installation.installed() {
        format!(
            "Up/Down or j/k: move   Space: toggle   a/n: all/no regular skills\nEnter: save changes   u: uninstall North   {leave}\nPipeline: commands + memory. [+]: required by enabled options."
        )
    } else {
        format!(
            "Up/Down or j/k: move   Space: toggle   a/n: all/no regular skills\nEnter: install North   {leave}\nPipeline: commands + memory. [+]: required by enabled options."
        )
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

    fn fixture() -> (tempfile::TempDir, Installation) {
        fixture_for(Tool::OpenCode)
    }

    fn fixture_for(tool: Tool) -> (tempfile::TempDir, Installation) {
        let repo = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .parent()
            .unwrap();
        let temp = tempfile::tempdir().unwrap();
        let installation = Installation::load(repo, tool, &temp.path().join("config")).unwrap();
        (temp, installation)
    }

    fn screen(terminal: &Terminal<TestBackend>) -> String {
        terminal
            .backend()
            .buffer()
            .content
            .iter()
            .map(|cell| cell.symbol())
            .collect()
    }

    #[test]
    fn workflows_are_grouped_separately_from_skills_and_installation() {
        let (_temp, installation) = fixture();
        let rows = entries(&installation);
        assert_eq!(
            &rows[..4],
            &[
                Entry::Heading("Workflow"),
                Entry::AutoCommit,
                Entry::Pipeline,
                Entry::Heading("Skills"),
            ]
        );
        assert_eq!(
            &rows[rows.len() - 4..],
            &[
                Entry::Heading("OpenSpec"),
                Entry::OpenSpec,
                Entry::Heading("Installation"),
                Entry::Merge,
            ]
        );
        let skills = regular_skills(&installation);
        assert!(!skills.contains(AUTO_COMMIT));
        assert!(!skills.contains(NORTH_PIPELINE));
        for name in PIPELINE_SKILLS {
            assert!(!skills.contains(*name));
        }
        assert!(skills.contains("clarify-requirements"));
        assert!(skills.contains("research"));
        assert!(skills.contains("subagent-usage"));
    }

    #[test]
    fn both_tools_offer_the_same_skill_catalog() {
        let (_temp, opencode) = fixture();
        let (_temp, claude) = fixture_for(Tool::Claude);
        assert_eq!(entries(&opencode), entries(&claude));
        assert_eq!(
            Entry::Merge.label(Tool::Claude),
            "Merge installations (import North into CLAUDE.md)"
        );
        assert_ne!(
            Entry::Merge.label(Tool::OpenCode),
            Entry::Merge.label(Tool::Claude)
        );
    }

    #[test]
    fn keyboard_navigation_skips_categories_and_wraps() {
        let (_temp, installation) = fixture();
        let rows = entries(&installation);
        let selectable: Vec<_> = rows
            .iter()
            .enumerate()
            .filter_map(|(index, entry)| entry.selectable().then_some(index))
            .collect();
        let mut list = ListState::default();
        for &index in &selectable {
            move_selection(&rows, &mut list, true);
            assert_eq!(list.selected(), Some(index));
        }
        move_selection(&rows, &mut list, true);
        assert_eq!(list.selected(), selectable.first().copied());
        for &index in selectable.iter().rev() {
            move_selection(&rows, &mut list, false);
            assert_eq!(list.selected(), Some(index));
        }
    }

    #[test]
    fn regular_skill_shortcuts_preserve_workflow_and_installation_choices() {
        let (_temp, installation) = fixture();
        let mut selected = installation.skill_names();
        let (mut openspec, mut merge) = (true, true);
        select_regular_skills(&installation, &mut selected, false);
        assert_eq!(
            selected,
            BTreeSet::from([AUTO_COMMIT.into(), NORTH_PIPELINE.into()])
        );
        let resolved = installation.resolved_skills(&selected).unwrap();
        for name in PIPELINE_SKILLS {
            assert!(resolved.contains(*name));
        }

        toggle_entry(&Entry::Pipeline, &mut selected, &mut openspec, &mut merge);
        assert_eq!(selected, BTreeSet::from([AUTO_COMMIT.into()]));
        assert_eq!(
            installation.resolved_skills(&selected).unwrap(),
            BTreeSet::from([AUTO_COMMIT.into()])
        );
        assert!(openspec && merge);

        select_regular_skills(&installation, &mut selected, true);
        assert!(!pipeline_enabled(&selected));
        assert!(selected.contains(AUTO_COMMIT));
        assert!(regular_skills(&installation).is_subset(&selected));
        toggle_entry(&Entry::Pipeline, &mut selected, &mut openspec, &mut merge);
        assert!(pipeline_enabled(&selected));
        toggle_entry(&Entry::AutoCommit, &mut selected, &mut openspec, &mut merge);
        let resolved = installation.resolved_skills(&selected).unwrap();
        assert!(resolved.contains("commit"));
        assert!(!resolved.contains(AUTO_COMMIT));
        assert!(pipeline_enabled(&selected));
        assert!(openspec && merge);
    }

    #[test]
    fn checklist_and_uninstall_confirmation_render_in_small_terminals() {
        for tool in Tool::ALL {
            let (_temp, installation) = fixture_for(tool);
            let rows = entries(&installation);
            for (width, height) in [(80, 24), (35, 12), (10, 5)] {
                let mut terminal = Terminal::new(TestBackend::new(width, height)).unwrap();
                for confirming in [false, true] {
                    let mut list = ListState::default();
                    for (index, entry) in rows.iter().enumerate() {
                        if !entry.selectable() {
                            continue;
                        }
                        list.select(Some(index));
                        terminal
                            .draw(|frame| {
                                render(
                                    frame,
                                    &installation,
                                    &installation.selected_skills(),
                                    &mut list,
                                    confirming,
                                    true,
                                    true,
                                    true,
                                )
                            })
                            .unwrap();
                        if width == 80 {
                            let text = screen(&terminal);
                            assert!(
                                text.contains(&format!("> [x] {}", entry.label(tool))),
                                "Selected option {} should be visible after scrolling",
                                entry.label(tool)
                            );
                            assert!(!text.contains("[x] commit"));
                            for name in PIPELINE_SKILLS {
                                assert!(!text.contains(&format!("[x] {name}")));
                            }
                            assert!(text.contains(if confirming {
                                "Confirm uninstall"
                            } else {
                                "Enter: install North"
                            }));
                            assert!(text.contains(tool.label()));
                            assert!(confirming || text.contains("Esc: choose another tool"));
                        }
                    }
                }
            }
        }
    }

    #[test]
    fn disabled_workflows_show_once_and_keep_confirmation_based_commit() {
        let (_temp, installation) = fixture();
        let mut terminal = Terminal::new(TestBackend::new(80, 24)).unwrap();
        terminal
            .draw(|frame| {
                render(
                    frame,
                    &installation,
                    &BTreeSet::new(),
                    &mut ListState::default().with_selected(Some(1)),
                    false,
                    false,
                    false,
                    false,
                )
            })
            .unwrap();
        let text = screen(&terminal);
        assert_eq!(text.matches("[ ] Auto commit").count(), 1);
        assert_eq!(text.matches("[ ] North pipeline").count(), 1);
        assert!(!text.contains("[ ] commit"));
        assert!(text.contains("Workflow"));
        assert!(text.contains("Skills"));
        assert!(text.contains("Options / 1 skills enabled"));
        assert!(text.contains("AGENTS-backup.md"));
        assert!(text.contains("q/Esc: quit without changes"));
        assert!(!text.contains("choose another tool"));
    }

    #[test]
    fn pipeline_shared_dependencies_are_marked_and_counted() {
        let (_temp, installation) = fixture();
        let selected = BTreeSet::from([NORTH_PIPELINE.into()]);
        let rows = entries(&installation);
        let mut terminal = Terminal::new(TestBackend::new(80, 24)).unwrap();
        for name in [
            "clarify-requirements",
            "research",
            "subagent-usage",
            "north-sources",
        ] {
            let index = rows
                .iter()
                .position(|entry| *entry == Entry::Skill(name.into()))
                .unwrap();
            terminal
                .draw(|frame| {
                    render(
                        frame,
                        &installation,
                        &selected,
                        &mut ListState::default().with_selected(Some(index)),
                        false,
                        false,
                        false,
                        false,
                    )
                })
                .unwrap();
            let text = screen(&terminal);
            assert!(text.contains(&format!("> [+] {name} (required)")));
            assert!(text.contains("Options / 10 skills enabled"));
            assert!(text.contains("[+]: required by enabled options."));
        }
        assert_eq!(selected, BTreeSet::from([NORTH_PIPELINE.into()]));
    }

    #[test]
    fn tool_picker_shows_status_and_blocks_broken_installations() {
        let (_temp, opencode) = fixture();
        let candidates = [
            Candidate {
                tool: Tool::OpenCode,
                installation: Ok(opencode),
            },
            Candidate {
                tool: Tool::Claude,
                installation: Err("Expected a real directory at /tmp/x".into()),
            },
        ];
        for (width, height) in [(80, 24), (35, 12), (10, 5)] {
            let mut terminal = Terminal::new(TestBackend::new(width, height)).unwrap();
            for index in 0..candidates.len() {
                let mut list = ListState::default().with_selected(Some(index));
                terminal
                    .draw(|frame| render_tools(frame, &candidates, &mut list))
                    .unwrap();
                if width == 80 {
                    let text = screen(&terminal);
                    assert!(text.contains("> OpenCode") == (index == 0));
                    assert!(text.contains("> Claude Code") == (index == 1));
                    assert!(text.contains("not installed"));
                    assert!(text.contains("unavailable: Expected a real directory"));
                    assert_eq!(
                        text.contains("Enter: continue with the selected tool"),
                        index == 0
                    );
                    assert_eq!(text.contains("cannot be managed"), index == 1);
                }
            }
        }
    }
}
