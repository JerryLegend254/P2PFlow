package modes

import (
	"fmt"
	"time"
)

// CollaborationMode represents different collaboration strategies
type CollaborationMode string

const (
	// RealtimeMode - immediate sync, high bandwidth, best for pair programming
	RealtimeMode CollaborationMode = "realtime"

	// BatchMode - periodic sync (configurable interval), reduced bandwidth
	BatchMode CollaborationMode = "batch"

	// ManualMode - no automatic sync, user triggers sync manually
	ManualMode CollaborationMode = "manual"

	// ReviewMode - changes are queued for review before applying
	ReviewMode CollaborationMode = "review"

	// ObserverMode - read-only, receives updates but cannot send changes
	ObserverMode CollaborationMode = "observer"

	// OfflineMode - work offline, sync when reconnected
	OfflineMode CollaborationMode = "offline"

	// LeaderMode - designated leader whose changes take precedence
	LeaderMode CollaborationMode = "leader"

	// FollowerMode - follows leader's changes, can make suggestions
	FollowerMode CollaborationMode = "follower"

	// ConflictFreeMode - uses CRDTs, eventual consistency
	ConflictFreeMode CollaborationMode = "conflict-free"

	// SelectiveMode - only sync specific files/directories
	SelectiveMode CollaborationMode = "selective"
)

// ConflictResolution defines how conflicts are handled
type ConflictResolution string

const (
	// ConflictLastWriteWins - timestamp-based, last write wins
	ConflictLastWriteWins ConflictResolution = "last-write-wins"

	// ConflictManual - require manual conflict resolution
	ConflictManual ConflictResolution = "manual"

	// ConflictCRDT - use CRDT-based automatic merge
	ConflictCRDT ConflictResolution = "crdt"

	// ConflictLeaderWins - leader's changes always win
	ConflictLeaderWins ConflictResolution = "leader-wins"

	// ConflictMerge - attempt automatic merge, fallback to manual
	ConflictMerge ConflictResolution = "merge"
)

// BandwidthProfile defines network usage patterns
type BandwidthProfile string

const (
	// BandwidthHigh - no restrictions, immediate transfer
	BandwidthHigh BandwidthProfile = "high"

	// BandwidthMedium - balanced approach with some throttling
	BandwidthMedium BandwidthProfile = "medium"

	// BandwidthLow - aggressive compression and throttling
	BandwidthLow BandwidthProfile = "low"

	// BandwidthMetered - minimal data transfer for metered connections
	BandwidthMetered BandwidthProfile = "metered"
)

// ChangeNotification defines how users are notified of changes
type ChangeNotification string

const (
	// NotifyAll - notify for every change
	NotifyAll ChangeNotification = "all"

	// NotifyImportant - only notify for important files
	NotifyImportant ChangeNotification = "important"

	// NotifyBatch - batch notifications
	NotifyBatch ChangeNotification = "batch"

	// NotifySilent - no notifications
	NotifySilent ChangeNotification = "silent"
)

// ModeConfig represents the configuration for a collaboration mode
type ModeConfig struct {
	// Core mode
	Mode CollaborationMode `json:"mode" yaml:"mode"`

	// Sync configuration
	RealtimeSync     bool          `json:"realtime_sync" yaml:"realtime_sync"`
	SyncInterval     time.Duration `json:"sync_interval" yaml:"sync_interval"`
	AutoSync         bool          `json:"auto_sync" yaml:"auto_sync"`
	SyncOnSave       bool          `json:"sync_on_save" yaml:"sync_on_save"`
	DebounceInterval time.Duration `json:"debounce_interval" yaml:"debounce_interval"`

	// Permissions
	ReadOnly          bool `json:"read_only" yaml:"read_only"`
	CanSendChanges    bool `json:"can_send_changes" yaml:"can_send_changes"`
	CanReceiveChanges bool `json:"can_receive_changes" yaml:"can_receive_changes"`
	RequireApproval   bool `json:"require_approval" yaml:"require_approval"`

	// Conflict handling
	ConflictStrategy ConflictResolution `json:"conflict_strategy" yaml:"conflict_strategy"`
	AllowMerge       bool               `json:"allow_merge" yaml:"allow_merge"`
	PreserveHistory  bool               `json:"preserve_history" yaml:"preserve_history"`

	// Network optimization
	BandwidthProfile BandwidthProfile `json:"bandwidth_profile" yaml:"bandwidth_profile"`
	UseCompression   bool             `json:"use_compression" yaml:"use_compression"`
	BatchOperations  bool             `json:"batch_operations" yaml:"batch_operations"`
	MaxBatchSize     int              `json:"max_batch_size" yaml:"max_batch_size"`
	ThrottleRate     int              `json:"throttle_rate" yaml:"throttle_rate"` // bytes per second, 0 = unlimited

	// Selective sync
	SelectivePaths []string `json:"selective_paths" yaml:"selective_paths"`
	ExcludePaths   []string `json:"exclude_paths" yaml:"exclude_paths"`

	// Notifications
	Notifications ChangeNotification `json:"notifications" yaml:"notifications"`
	VerboseOutput bool               `json:"verbose_output" yaml:"verbose_output"`

	// Leader/follower
	IsLeader     bool   `json:"is_leader" yaml:"is_leader"`
	LeaderPeerID string `json:"leader_peer_id" yaml:"leader_peer_id"`

	// Advanced features
	EnableAntiEntropy   bool          `json:"enable_anti_entropy" yaml:"enable_anti_entropy"`
	AntiEntropyInterval time.Duration `json:"anti_entropy_interval" yaml:"anti_entropy_interval"`
	EnablePrefetch      bool          `json:"enable_prefetch" yaml:"enable_prefetch"`
	CacheChanges        bool          `json:"cache_changes" yaml:"cache_changes"`
}

// Predefined mode configurations
var PresetModes = map[CollaborationMode]ModeConfig{
	RealtimeMode: {
		Mode:                RealtimeMode,
		RealtimeSync:        true,
		SyncInterval:        100 * time.Millisecond,
		AutoSync:            true,
		SyncOnSave:          true,
		DebounceInterval:    50 * time.Millisecond,
		ReadOnly:            false,
		CanSendChanges:      true,
		CanReceiveChanges:   true,
		RequireApproval:     false,
		ConflictStrategy:    ConflictLastWriteWins,
		AllowMerge:          true,
		PreserveHistory:     true,
		BandwidthProfile:    BandwidthHigh,
		UseCompression:      false,
		BatchOperations:     false,
		MaxBatchSize:        1,
		ThrottleRate:        0,
		Notifications:       NotifyAll,
		VerboseOutput:       true,
		EnableAntiEntropy:   true,
		AntiEntropyInterval: 10 * time.Second,
		EnablePrefetch:      true,
		CacheChanges:        true,
	},

	BatchMode: {
		Mode:                BatchMode,
		RealtimeSync:        false,
		SyncInterval:        5 * time.Second,
		AutoSync:            true,
		SyncOnSave:          false,
		DebounceInterval:    2 * time.Second,
		ReadOnly:            false,
		CanSendChanges:      true,
		CanReceiveChanges:   true,
		RequireApproval:     false,
		ConflictStrategy:    ConflictMerge,
		AllowMerge:          true,
		PreserveHistory:     true,
		BandwidthProfile:    BandwidthMedium,
		UseCompression:      true,
		BatchOperations:     true,
		MaxBatchSize:        50,
		ThrottleRate:        1024 * 1024, // 1 MB/s
		Notifications:       NotifyBatch,
		VerboseOutput:       false,
		EnableAntiEntropy:   true,
		AntiEntropyInterval: 30 * time.Second,
		EnablePrefetch:      false,
		CacheChanges:        true,
	},

	ManualMode: {
		Mode:                ManualMode,
		RealtimeSync:        false,
		SyncInterval:        0,
		AutoSync:            false,
		SyncOnSave:          false,
		DebounceInterval:    0,
		ReadOnly:            false,
		CanSendChanges:      true,
		CanReceiveChanges:   true,
		RequireApproval:     false,
		ConflictStrategy:    ConflictManual,
		AllowMerge:          true,
		PreserveHistory:     true,
		BandwidthProfile:    BandwidthMedium,
		UseCompression:      true,
		BatchOperations:     true,
		MaxBatchSize:        100,
		ThrottleRate:        0,
		Notifications:       NotifyImportant,
		VerboseOutput:       true,
		EnableAntiEntropy:   false,
		AntiEntropyInterval: 0,
		EnablePrefetch:      false,
		CacheChanges:        true,
	},

	ReviewMode: {
		Mode:                ReviewMode,
		RealtimeSync:        false,
		SyncInterval:        0,
		AutoSync:            false,
		SyncOnSave:          false,
		DebounceInterval:    1 * time.Second,
		ReadOnly:            false,
		CanSendChanges:      true,
		CanReceiveChanges:   true,
		RequireApproval:     true,
		ConflictStrategy:    ConflictManual,
		AllowMerge:          false,
		PreserveHistory:     true,
		BandwidthProfile:    BandwidthMedium,
		UseCompression:      true,
		BatchOperations:     true,
		MaxBatchSize:        20,
		ThrottleRate:        512 * 1024, // 512 KB/s
		Notifications:       NotifyImportant,
		VerboseOutput:       true,
		EnableAntiEntropy:   false,
		AntiEntropyInterval: 0,
		EnablePrefetch:      false,
		CacheChanges:        true,
	},

	ObserverMode: {
		Mode:                ObserverMode,
		RealtimeSync:        true,
		SyncInterval:        500 * time.Millisecond,
		AutoSync:            true,
		SyncOnSave:          false,
		DebounceInterval:    500 * time.Millisecond,
		ReadOnly:            true,
		CanSendChanges:      false,
		CanReceiveChanges:   true,
		RequireApproval:     false,
		ConflictStrategy:    ConflictLastWriteWins,
		AllowMerge:          false,
		PreserveHistory:     false,
		BandwidthProfile:    BandwidthMedium,
		UseCompression:      true,
		BatchOperations:     true,
		MaxBatchSize:        10,
		ThrottleRate:        256 * 1024, // 256 KB/s
		Notifications:       NotifyBatch,
		VerboseOutput:       false,
		EnableAntiEntropy:   true,
		AntiEntropyInterval: 30 * time.Second,
		EnablePrefetch:      true,
		CacheChanges:        false,
	},

	OfflineMode: {
		Mode:                OfflineMode,
		RealtimeSync:        false,
		SyncInterval:        0,
		AutoSync:            false,
		SyncOnSave:          false,
		DebounceInterval:    0,
		ReadOnly:            false,
		CanSendChanges:      true,
		CanReceiveChanges:   false,
		RequireApproval:     false,
		ConflictStrategy:    ConflictCRDT,
		AllowMerge:          true,
		PreserveHistory:     true,
		BandwidthProfile:    BandwidthLow,
		UseCompression:      true,
		BatchOperations:     true,
		MaxBatchSize:        1000,
		ThrottleRate:        0,
		Notifications:       NotifySilent,
		VerboseOutput:       false,
		EnableAntiEntropy:   false,
		AntiEntropyInterval: 0,
		EnablePrefetch:      false,
		CacheChanges:        true,
	},

	LeaderMode: {
		Mode:                LeaderMode,
		RealtimeSync:        true,
		SyncInterval:        200 * time.Millisecond,
		AutoSync:            true,
		SyncOnSave:          true,
		DebounceInterval:    100 * time.Millisecond,
		ReadOnly:            false,
		CanSendChanges:      true,
		CanReceiveChanges:   false, // Leader doesn't accept changes
		RequireApproval:     false,
		ConflictStrategy:    ConflictLeaderWins,
		AllowMerge:          false,
		PreserveHistory:     true,
		BandwidthProfile:    BandwidthHigh,
		UseCompression:      false,
		BatchOperations:     false,
		MaxBatchSize:        1,
		ThrottleRate:        0,
		IsLeader:            true,
		Notifications:       NotifyAll,
		VerboseOutput:       true,
		EnableAntiEntropy:   true,
		AntiEntropyInterval: 5 * time.Second,
		EnablePrefetch:      false,
		CacheChanges:        false,
	},

	FollowerMode: {
		Mode:                FollowerMode,
		RealtimeSync:        true,
		SyncInterval:        200 * time.Millisecond,
		AutoSync:            true,
		SyncOnSave:          false, // Don't send saves automatically
		DebounceInterval:    500 * time.Millisecond,
		ReadOnly:            false,
		CanSendChanges:      true, // Can send suggestions
		CanReceiveChanges:   true,
		RequireApproval:     true, // Suggestions need approval
		ConflictStrategy:    ConflictLeaderWins,
		AllowMerge:          false,
		PreserveHistory:     true,
		BandwidthProfile:    BandwidthMedium,
		UseCompression:      true,
		BatchOperations:     true,
		MaxBatchSize:        10,
		ThrottleRate:        512 * 1024, // 512 KB/s
		IsLeader:            false,
		Notifications:       NotifyImportant,
		VerboseOutput:       false,
		EnableAntiEntropy:   true,
		AntiEntropyInterval: 10 * time.Second,
		EnablePrefetch:      true,
		CacheChanges:        true,
	},

	ConflictFreeMode: {
		Mode:                ConflictFreeMode,
		RealtimeSync:        true,
		SyncInterval:        300 * time.Millisecond,
		AutoSync:            true,
		SyncOnSave:          true,
		DebounceInterval:    200 * time.Millisecond,
		ReadOnly:            false,
		CanSendChanges:      true,
		CanReceiveChanges:   true,
		RequireApproval:     false,
		ConflictStrategy:    ConflictCRDT,
		AllowMerge:          true,
		PreserveHistory:     true,
		BandwidthProfile:    BandwidthMedium,
		UseCompression:      true,
		BatchOperations:     false,
		MaxBatchSize:        1,
		ThrottleRate:        0,
		Notifications:       NotifyImportant,
		VerboseOutput:       true,
		EnableAntiEntropy:   true,
		AntiEntropyInterval: 15 * time.Second,
		EnablePrefetch:      true,
		CacheChanges:        true,
	},

	SelectiveMode: {
		Mode:                SelectiveMode,
		RealtimeSync:        true,
		SyncInterval:        500 * time.Millisecond,
		AutoSync:            true,
		SyncOnSave:          true,
		DebounceInterval:    300 * time.Millisecond,
		ReadOnly:            false,
		CanSendChanges:      true,
		CanReceiveChanges:   true,
		RequireApproval:     false,
		ConflictStrategy:    ConflictMerge,
		AllowMerge:          true,
		PreserveHistory:     true,
		BandwidthProfile:    BandwidthMedium,
		UseCompression:      true,
		BatchOperations:     true,
		MaxBatchSize:        20,
		ThrottleRate:        1024 * 1024, // 1 MB/s
		SelectivePaths:      []string{},  // User must specify
		Notifications:       NotifyImportant,
		VerboseOutput:       false,
		EnableAntiEntropy:   true,
		AntiEntropyInterval: 20 * time.Second,
		EnablePrefetch:      false,
		CacheChanges:        true,
	},
}

// GetModeConfig returns the configuration for a given mode
func GetModeConfig(mode CollaborationMode) (ModeConfig, error) {
	config, exists := PresetModes[mode]
	if !exists {
		return ModeConfig{}, fmt.Errorf("unknown collaboration mode: %s", mode)
	}
	return config, nil
}

// GetAvailableModes returns a list of all available modes
func GetAvailableModes() []CollaborationMode {
	return []CollaborationMode{
		RealtimeMode,
		BatchMode,
		ManualMode,
		ReviewMode,
		ObserverMode,
		OfflineMode,
		LeaderMode,
		FollowerMode,
		ConflictFreeMode,
		SelectiveMode,
	}
}

// GetModeDescription returns a human-readable description of a mode
func GetModeDescription(mode CollaborationMode) string {
	descriptions := map[CollaborationMode]string{
		RealtimeMode:     "Immediate sync with high bandwidth - best for pair programming",
		BatchMode:        "Periodic sync with reduced bandwidth - balanced for teams",
		ManualMode:       "No automatic sync - full control over when to sync",
		ReviewMode:       "Changes queued for review - code review workflow",
		ObserverMode:     "Read-only mode - watch others work without editing",
		OfflineMode:      "Work offline, sync when reconnected - intermittent connection",
		LeaderMode:       "Designated leader - your changes take precedence",
		FollowerMode:     "Follow leader's changes - make suggestions",
		ConflictFreeMode: "CRDT-based eventual consistency - automatic conflict resolution",
		SelectiveMode:    "Sync only specific files/directories - focused collaboration",
	}
	return descriptions[mode]
}

// ValidateConfig validates a mode configuration
func (mc *ModeConfig) ValidateConfig() error {
	if mc.Mode == "" {
		return fmt.Errorf("mode cannot be empty")
	}

	if mc.ReadOnly && mc.CanSendChanges {
		return fmt.Errorf("read-only mode cannot send changes")
	}

	if mc.IsLeader && mc.CanReceiveChanges {
		return fmt.Errorf("leader mode should not receive changes from followers")
	}

	if mc.RealtimeSync && mc.SyncInterval == 0 {
		return fmt.Errorf("realtime sync requires a sync interval")
	}

	if mc.Mode == SelectiveMode && len(mc.SelectivePaths) == 0 {
		return fmt.Errorf("selective mode requires at least one selective path")
	}

	return nil
}

// CustomizeConfig allows customization of a preset mode
func (mc *ModeConfig) CustomizeConfig(customizations map[string]interface{}) error {
	// Apply custom settings on top of preset
	for key, value := range customizations {
		switch key {
		case "sync_interval":
			if duration, ok := value.(time.Duration); ok {
				mc.SyncInterval = duration
			}
		case "bandwidth_profile":
			if profile, ok := value.(BandwidthProfile); ok {
				mc.BandwidthProfile = profile
			}
		case "notifications":
			if notif, ok := value.(ChangeNotification); ok {
				mc.Notifications = notif
			}
		case "selective_paths":
			if paths, ok := value.([]string); ok {
				mc.SelectivePaths = paths
			}
		case "throttle_rate":
			if rate, ok := value.(int); ok {
				mc.ThrottleRate = rate
			}
		}
	}

	return mc.ValidateConfig()
}

// ShouldSync determines if sync should happen based on mode config
func (mc *ModeConfig) ShouldSync() bool {
	if !mc.AutoSync {
		return false
	}

	if mc.ReadOnly {
		return false
	}

	return mc.CanSendChanges
}

// ShouldReceive determines if changes should be received
func (mc *ModeConfig) ShouldReceive() bool {
	return mc.CanReceiveChanges
}

// ShouldNotify determines if a notification should be shown
func (mc *ModeConfig) ShouldNotify(isImportant bool) bool {
	switch mc.Notifications {
	case NotifyAll:
		return true
	case NotifyImportant:
		return isImportant
	case NotifyBatch, NotifySilent:
		return false
	default:
		return false
	}
}
