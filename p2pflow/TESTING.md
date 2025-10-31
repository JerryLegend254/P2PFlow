# Testing P2P Collaboration Engine

This guide shows you how to test Objective 3 (shared state management and patch merging) locally.

## 🚀 Quick Start

### Option 1: Automated Test Script
```bash
# Run the automated test script
./scripts/test-collab.sh
```

### Option 2: Manual Testing

#### Step 1: Build the Components
```bash
# Build the collaboration engine test
go build -o bin/test-collab ./cmd/test-collab

# Build the file watcher test
go build -o bin/test-watcher ./cmd/test-watcher

# Build the original watcher
go build -o bin/p2pflow-watcher ./cmd/watcher
```

#### Step 2: Test Collaboration Engine Directly
```bash
# Run collaboration engine tests
./bin/test-collab
```

This will test:
- ✅ Single agent patch generation
- ✅ Multiple agents concurrent edits
- ✅ Conflict resolution
- ✅ Session persistence

#### Step 3: Test File Watcher Integration
```bash
# Terminal 1: Start the watcher
./bin/test-watcher

# Terminal 2: Edit the test file
echo "New content added" >> test-watcher.txt
```

## 🔍 What to Look For

### 1. Patch Generation
- Watch for "Patch generated:" messages showing diff patches
- Patches should be in standard diff format with `@@` markers

### 2. Session Management
- Session ID and Agent ID should be generated
- Version numbers should increment with each change
- Session metadata should be saved to `.collab/` directory

### 3. Collaboration Engine Features
- Multiple agents can join the same session
- Changes are applied sequentially with version tracking
- Conflict detection for overlapping edits
- Automatic conflict resolution

### 4. Persistence
- Session data saved to `.collab/session_*.json`
- Change events saved to `.collab/events/event_*.json`
- Data persists between runs

## 📁 File Structure After Testing

```
.collab/
├── session_<session-id>.json    # Session metadata
└── events/
    └── event_<session-id>_<timestamp>.json  # Change events
```

## 🐛 Troubleshooting

### Common Issues

1. **Build Errors**: Make sure all dependencies are installed
   ```bash
   go mod tidy
   ```

2. **Permission Errors**: Make sure the test script is executable
   ```bash
   chmod +x scripts/test-collab.sh
   ```

3. **File Not Found**: Make sure you're in the project root directory

### Debug Mode

To see more detailed output, check the logs in the test output. The collaboration engine will show:
- Session creation and agent joining
- Change application and version updates
- Conflict detection and resolution
- File persistence operations

## 🎯 Expected Results

When working correctly, you should see:

1. **Single Agent Test**: 3 successful changes with version increments
2. **Multiple Agents Test**: 3 agents joining and making concurrent edits
3. **Conflict Resolution Test**: Conflicts detected and resolved automatically
4. **Session Persistence Test**: Session data and change events saved to disk

## 🔄 Testing Concurrent Edits

To test concurrent edits manually:

1. Start the watcher: `./bin/test-watcher`
2. In another terminal, rapidly edit the file:
   ```bash
   echo "Edit 1" >> test-watcher.txt
   echo "Edit 2" >> test-watcher.txt
   echo "Edit 3" >> test-watcher.txt
   ```
3. Watch the collaboration engine handle the changes
4. Check `.collab/` directory for persisted data

This validates that Objective 3 (shared state management and patch merging) is working correctly!
