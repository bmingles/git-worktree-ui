# Performance Optimization Plan

## Project Context

**Project**: Git Worktree TUI Manager  
**Repository**: bmingles/git-worktree-ui  
**Language**: Go  
**Framework**: Bubble Tea (terminal UI)  
**Issue**: Application becomes sluggish to load as the number of configured projects increases

### Project Structure Overview

The TUI application manages Git worktrees across multiple projects:

- **Main Entry**: `wt/main.go` → `wt/cmd/root.go` → `wt/pkg/tui/model.go`
- **Key Packages**:
  - `pkg/tui/` - Bubble Tea TUI model and view
  - `pkg/worktree/` - Git worktree operations
  - `pkg/config/` - Configuration management
  - `pkg/vscode/` - VS Code integration
  - `pkg/workspace/` - Workspace file handling

### Current Architecture

1. **Startup Flow** (`cmd/root.go:launchTUI()`):
   - Load config file (`config.LoadConfig()`)
   - Create TUI model (`tui.NewModel()`)
   - During `NewModel()`:
     - Calls `buildItems()` - builds navigation list
     - Calls `loadWorktrees()` - **BLOCKS HERE**
   - Launch Bubble Tea program

2. **Data Loading** (`pkg/tui/model.go:loadWorktrees()`):
   - Iterates through ALL projects sequentially
   - For each project:
     - Executes `git worktree list --porcelain`
     - For each worktree found:
       - Executes `git status --porcelain --branch`
   - **All operations are synchronous and blocking**

3. **UI Rendering** (`pkg/tui/view.go`):
   - Displays projects in categories
   - Shows worktree count and status indicators:
     - `●` - Primary worktree
     - `*` - Uncommitted changes
     - `↑N` - Commits ahead
     - `↓N` - Commits behind

---

## Performance Bottlenecks Identified

### 🔴 CRITICAL #1: Synchronous Sequential Git Operations on Startup

**Location**: `pkg/tui/model.go:1105-1125`

```go
func (m *Model) loadWorktrees() {
    for _, project := range m.projects {
        wts, err := worktree.ListWorktrees(project.Path)
        if err != nil {
            m.err = err
            continue
        }

        // Load status for each worktree
        for i := range wts {
            status, err := worktree.GetStatus(wts[i].Path)
            if err != nil {
                // Continue with empty status on error
                continue
            }
            wts[i].Status = status
        }

        m.worktrees[project.Path] = wts
    }
    m.buildItems()
}
```

**Problem**:

- Called synchronously in `NewModel()` before UI appears
- Executes `git worktree list` for every project sequentially
- Executes `git status` for every worktree sequentially
- No parallelization or async loading

**Impact Calculation**:

- 10 projects with 5 worktrees each = 60 total git commands
- Each git command: 50-200ms (depending on repo size)
- **Total startup delay: 3-12 seconds** before UI appears
- Scales linearly with project count (O(n\*m) where n=projects, m=worktrees)

---

### 🟡 MEDIUM #2: No Lazy Loading for Collapsed Projects

**Location**: `pkg/tui/model.go:860-1000` (buildItems) and view rendering

**Problem**:

- All worktrees loaded upfront, even for collapsed projects
- Git status information collected but not displayed until project is expanded
- `expandedProjects` map tracks expansion state but doesn't affect data loading
- Status indicators only visible when project is expanded

**Current Behavior**:

```go
// In NewModel() - all projects default to collapsed
for _, project := range projects {
    m.expandedProjects[project.Path] = false
}

// But loadWorktrees() still loads everything
m.loadWorktrees()
```

**Impact**:

- Wasted CPU/IO loading data that isn't immediately visible
- User waits for all data before seeing anything
- Memory usage higher than necessary

---

### 🟡 MEDIUM #3: BuildItems() Called on Every Search Keystroke

**Location**: `pkg/tui/model.go:192-200`

```go
default:
    // Update search input and apply filter in real-time
    var cmd tea.Cmd
    m.searchInput, cmd = m.searchInput.Update(msg)
    m.filterTerm = m.searchInput.Value()
    m.buildItems()  // Called on EVERY keystroke
    return m, cmd
```

**Problem**:

- `buildItems()` rebuilds entire navigation list on every character typed
- Includes nested loops over categories, projects, worktrees
- Runs filtering logic on every keystroke

**Impact**:

- Noticeable lag when typing search queries with many projects
- CPU churn during interactive search
- Not critical (data is pre-loaded) but reduces UI responsiveness

---

### 🟢 LOW #4: No Caching of Git Status

**Related to**: Issue #1 and #2

**Problem**:

- Git status is fetched once at startup but never refreshed
- No TTL or cache invalidation strategy
- If implementing lazy loading, repeated expansions would re-query

**Impact**:

- Currently mitigated by only loading once
- Will become relevant when implementing lazy loading

---

## Optimization Strategy

### Phase 1: Async Loading with Immediate UI (HIGH PRIORITY) 🎯

**Goal**: Show UI immediately, load data in background

**Implementation Steps**:

1. **Modify NewModel() to skip synchronous load**:

   ```go
   func NewModel(projects []config.Project, categories []string) Model {
       // ... existing setup ...

       m.buildItems() // Build with empty worktrees
       // DON'T call m.loadWorktrees() here

       return m
   }
   ```

2. **Return async load command from Init()**:

   ```go
   func (m Model) Init() tea.Cmd {
       return m.loadAllWorktreesAsync()
   }
   ```

3. **Create async loading infrastructure**:

   ```go
   // Message types
   type worktreesLoadedMsg struct {
       projectPath string
       worktrees   []worktree.Worktree
       err         error
   }

   type allWorktreesLoadedMsg struct{}

   // Async loader
   func (m Model) loadAllWorktreesAsync() tea.Cmd {
       return func() tea.Msg {
           type result struct {
               projectPath string
               worktrees   []worktree.Worktree
               err         error
           }

           results := make(chan result, len(m.projects))

           // Load projects in parallel (with concurrency limit)
           sem := make(chan struct{}, 5) // Max 5 concurrent
           for _, project := range m.projects {
               go func(p config.Project) {
                   sem <- struct{}{} // Acquire
                   defer func() { <-sem }() // Release

                   wts, err := worktree.ListWorktrees(p.Path)
                   if err != nil {
                       results <- result{p.Path, nil, err}
                       return
                   }

                   // Load status for each worktree
                   for i := range wts {
                       status, _ := worktree.GetStatus(wts[i].Path)
                       wts[i].Status = status
                   }

                   results <- result{p.Path, wts, nil}
               }(project)
           }

           // Collect all results
           for i := 0; i < len(m.projects); i++ {
               res := <-results
               // Send individual messages or batch?
               // Could send updates as they complete for progressive loading
           }

           return allWorktreesLoadedMsg{}
       }
   }
   ```

4. **Handle loading messages in Update()**:

   ```go
   case worktreesLoadedMsg:
       m.worktrees[msg.projectPath] = msg.worktrees
       m.buildItems()
       return m, nil

   case allWorktreesLoadedMsg:
       // All loaded, maybe clear a "loading..." indicator
       return m, nil
   ```

5. **Add loading indicator to View()**:
   ```go
   if len(m.worktrees) < len(m.projects) {
       b.WriteString(helpStyle.Render("Loading worktrees..."))
   }
   ```

**Expected Improvement**:

- UI appears in < 100ms instead of 3-12 seconds
- Projects loaded in parallel (5x speedup with 5 concurrent workers)
- Progressive updates as data loads

---

### Phase 2: Lazy Loading for Collapsed Projects (MEDIUM PRIORITY)

**Goal**: Only load worktree details when user expands a project

**Implementation Steps**:

1. **Track loading state**:

   ```go
   type Model struct {
       // ... existing fields ...
       worktreesLoading map[string]bool // Track which projects are being loaded
       worktreesLoaded  map[string]bool // Track which projects have been loaded
   }
   ```

2. **Modify initial load to only fetch worktree count**:

   ```go
   // Quick count without full status
   func CountWorktrees(projectPath string) (int, error) {
       cmd := exec.Command("git", "worktree", "list")
       output, err := cmd.CombinedOutput()
       if err != nil {
           return 0, err
       }
       return len(strings.Split(strings.TrimSpace(string(output)), "\n")), nil
   }
   ```

3. **Load full details on expansion**:
   ```go
   case "space", "enter":
       if item.Type == ItemTypeProject {
           // Toggle expansion
           m.expandedProjects[item.ProjectPath] = !m.expandedProjects[item.ProjectPath]

           // Load worktrees if expanding and not loaded
           if m.expandedProjects[item.ProjectPath] && !m.worktreesLoaded[item.ProjectPath] {
               return m, m.loadProjectWorktrees(item.ProjectPath)
           }

           m.buildItems()
           return m, nil
       }
   ```

**Expected Improvement**:

- Initial load even faster (just project names + counts)
- Data loaded on-demand
- Better scaling for large numbers of projects

---

### Phase 3: Search Debouncing (LOW PRIORITY)

**Goal**: Reduce unnecessary rebuilds during search

**Implementation**:

1. **Add debounce timer**:

   ```go
   type Model struct {
       // ... existing fields ...
       searchDebounceTimer *time.Timer
   }
   ```

2. **Debounce in search handler**:
   ```go
   default:
       var cmd tea.Cmd
       m.searchInput, cmd = m.searchInput.Update(msg)
       m.filterTerm = m.searchInput.Value()

       // Don't rebuild immediately, wait for pause in typing
       // OR just rebuild on Enter/navigation only

       return m, cmd
   ```

**Alternative**: Only apply filter on Enter, show preview during typing

**Expected Improvement**:

- Smoother typing experience with many projects
- Reduced CPU usage during search

---

### Phase 4: Status Caching (FUTURE)

**Goal**: Cache git status with TTL, allow manual refresh

**Implementation**:

```go
type StatusCache struct {
    status    GitStatus
    timestamp time.Time
}

func (m Model) getWorktreeStatus(path string, maxAge time.Duration) GitStatus {
    if cached, ok := m.statusCache[path]; ok {
        if time.Since(cached.timestamp) < maxAge {
            return cached.status
        }
    }

    // Fetch fresh status
    status, _ := worktree.GetStatus(path)
    m.statusCache[path] = StatusCache{status, time.Now()}
    return status
}
```

Add refresh command (e.g., 'r' key) to reload status for visible worktrees.

---

## Implementation Priority

### Must Have (Week 1):

- ✅ Phase 1: Async loading with immediate UI
- ✅ Add loading indicators
- ✅ Graceful error handling for failed git operations

### Should Have (Week 2):

- ✅ Phase 2: Lazy loading for collapsed projects
- ✅ Concurrent git operations with proper limits
- ✅ Progress indicators for batch operations

### Nice to Have (Week 3):

- ✅ Phase 3: Search debouncing
- ✅ Phase 4: Status caching with refresh
- ✅ Configurable concurrency limits
- ✅ Performance metrics/logging

---

## Testing Strategy

### Performance Benchmarks:

Create test scenarios:

1. **Small**: 5 projects, 3 worktrees each (~15 total)
2. **Medium**: 20 projects, 5 worktrees each (~100 total)
3. **Large**: 50 projects, 10 worktrees each (~500 total)

Measure:

- Time to first UI render
- Time to fully loaded state
- Memory usage
- CPU usage during operations

### Test Cases:

1. **Startup Performance**:
   - Timer from program start to UI visible
   - Timer from UI visible to data fully loaded

2. **Interaction Performance**:
   - Expand/collapse responsiveness
   - Search typing lag
   - Navigation smoothness

3. **Error Handling**:
   - Non-git directories
   - Corrupted repositories
   - Missing permissions
   - Concurrent access issues

---

## Code References

### Files to Modify:

1. **`pkg/tui/model.go`** (Primary changes):
   - `NewModel()` - Remove synchronous `loadWorktrees()` call
   - `Init()` - Return async loading command
   - `Update()` - Handle new loading messages
   - `loadWorktrees()` - Convert to async or remove
   - Add new methods:
     - `loadAllWorktreesAsync()`
     - `loadProjectWorktrees(projectPath)`
     - `handleWorktreesLoaded(msg)`

2. **`pkg/tui/view.go`** (Minor changes):
   - Add loading indicators
   - Handle partial data states
   - Show loading spinner/progress

3. **`pkg/worktree/worktree.go`** (Optional):
   - Add `CountWorktrees()` helper
   - Consider batch operations

### New Message Types to Add:

```go
// In pkg/tui/model.go

type worktreeLoadStartMsg struct {
    projectPath string
}

type worktreeLoadCompleteMsg struct {
    projectPath string
    worktrees   []worktree.Worktree
    err         error
}

type allWorktreesLoadedMsg struct {
    totalProjects int
    totalWorktrees int
    errors []error
}
```

---

## Risk Assessment

### Low Risk:

- Phase 1 (Async loading) - Well-isolated change, easy to test
- Phase 3 (Search debouncing) - UI-only change

### Medium Risk:

- Phase 2 (Lazy loading) - Changes data loading contract, needs careful testing
- Concurrency limits - Need to tune for different systems

### Mitigation:

- Feature flag for new loading behavior
- Fallback to synchronous on error
- Comprehensive error handling
- User feedback during long operations

---

## Success Criteria

### Performance Goals:

- ✅ UI appears in < 200ms regardless of project count
- ✅ Full load time < 2 seconds for 50 projects
- ✅ No UI lag during typing/navigation
- ✅ Memory usage reasonable (< 50MB for typical usage)

### User Experience Goals:

- ✅ Immediate responsiveness
- ✅ Clear loading indicators
- ✅ Graceful degradation on errors
- ✅ No breaking changes to existing functionality

---

## Future Enhancements

Beyond immediate performance fixes:

1. **Incremental Updates**: Watch for filesystem changes, update in background
2. **Persistent Cache**: Save worktree data to disk, load instantly on startup
3. **Virtual Scrolling**: For hundreds of projects, only render visible items
4. **Background Refresh**: Periodic status updates for active worktrees
5. **Bulk Operations**: Parallel fetch/pull for multiple worktrees

---

## Notes for Implementation

- Bubble Tea is single-threaded - use tea.Cmd for async operations
- Channel-based communication for goroutines
- Careful with shared state in `Model` - only modify in `Update()`
- Test with real repos of varying sizes
- Consider making concurrency limit configurable
- Add telemetry/logging for debugging performance issues

---

**Document Version**: 1.0  
**Last Updated**: March 26, 2026  
**Status**: Ready for Implementation
