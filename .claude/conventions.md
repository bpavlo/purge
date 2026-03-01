# Development Conventions

## Parallel Work: Git Worktrees

When working on multiple features in parallel, use **real git worktrees** — not subdirectories.

### Setup

```bash
# From the main repo at /home/pavlo/purge/
# Create a worktree for a feature branch:
git worktree add ../purge-<feature> -b feature/<feature>

# Example:
git worktree add ../purge-discord -b feature/discord
git worktree add ../purge-telegram -b feature/telegram
```

This creates sibling directories that are full checkouts of different branches:
```
/home/pavlo/
├── purge/                  # main branch (primary repo)
├── purge-discord/          # feature/discord branch (worktree)
└── purge-telegram/         # feature/telegram branch (worktree)
```

### Rules

1. **Never create code subdirectories** like `worktrees/` inside the repo — that duplicates files
2. Each worktree is a **sibling directory** (e.g. `../purge-<name>`)
3. Each worktree checks out its own **branch** (e.g. `feature/<name>`)
4. When done, merge the branch and remove the worktree:
   ```bash
   git merge feature/<name>
   git worktree remove ../purge-<name>
   git branch -d feature/<name>
   ```
5. Agents working in parallel get assigned **different worktree paths** — never the same directory

### Branch naming

- `feature/<name>` — new features
- `fix/<name>` — bug fixes
- `refactor/<name>` — refactoring
