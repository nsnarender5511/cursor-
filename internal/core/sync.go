package core

import (
	"fmt"
	"os"
	"path/filepath"

	"crules/internal/git"
	"crules/internal/ui"
	"crules/internal/utils"
)

// SyncManager handles all sync operations
type SyncManager struct {
	mainPath string
	registry *Registry
	config   *utils.Config
	appPaths utils.AppPaths
}

// NewSyncManager creates a new sync manager
func NewSyncManager() (*SyncManager, error) {
	// Load config
	config := utils.LoadConfig()
	utils.Debug("Loaded configuration")

	// Get app name from environment
	appName := os.Getenv("APP_NAME")
	if appName == "" {
		appName = utils.DefaultAppName
	}

	// Get system paths
	appPaths := utils.GetAppPaths(appName)
	utils.Debug("Loaded system paths for current platform")

	// Use OS-specific paths
	mainPath := appPaths.GetRulesDir(config.RulesDirName)
	registryPath := appPaths.GetRegistryFile(config.RegistryFileName)

	// Ensure required directories exist
	if err := utils.EnsureDirExists(appPaths.ConfigDir, config.DirPermission); err != nil {
		utils.Error("Cannot create config directory | path=" + appPaths.ConfigDir + ", error=" + err.Error())
		return nil, fmt.Errorf("cannot create config directory: %v", err)
	}

	if err := utils.EnsureDirExists(appPaths.DataDir, config.DirPermission); err != nil {
		utils.Error("Cannot create data directory | path=" + appPaths.DataDir + ", error=" + err.Error())
		return nil, fmt.Errorf("cannot create data directory: %v", err)
	}

	if err := utils.EnsureDirExists(appPaths.LogDir, config.DirPermission); err != nil {
		utils.Error("Cannot create log directory | path=" + appPaths.LogDir + ", error=" + err.Error())
		return nil, fmt.Errorf("cannot create log directory: %v", err)
	}

	utils.Debug("Using main rules path | path=" + mainPath)

	// Ensure main directory exists
	if err := os.MkdirAll(mainPath, config.DirPermission); err != nil {
		utils.Error("Cannot create main directory | path=" + mainPath + ", error=" + err.Error())
		return nil, fmt.Errorf("cannot create main directory: %v", err)
	}

	// Load or create registry
	registry, err := LoadRegistry(registryPath, config)
	if err != nil {
		utils.Error("Cannot load registry | error=" + err.Error())
		return nil, fmt.Errorf("cannot load registry: %v", err)
	}
	utils.Debug("Registry loaded successfully")

	return &SyncManager{
		mainPath: mainPath,
		registry: registry,
		config:   config,
		appPaths: appPaths,
	}, nil
}

// Init copies rules from main location to current directory
func (sm *SyncManager) Init() error {
	currentDir, err := os.Getwd()
	if err != nil {
		utils.Error("Cannot get current directory | error=" + err.Error())
		return fmt.Errorf("cannot get current directory: %v", err)
	}

	targetPath := filepath.Join(currentDir, sm.config.RulesDirName)
	utils.Debug("Init target path | path=" + targetPath)

	// Check if the main rules location exists
	mainLocationNeedsSetup := false

	if !utils.DirExists(sm.mainPath) {
		utils.Debug("Main rules location does not exist | path=" + sm.mainPath)
		ui.Warning("Main rules location does not exist: %s", sm.mainPath)
		mainLocationNeedsSetup = true
	} else {
		// Main location exists, but check if it has any .mdc files
		hasMDCFiles, err := utils.HasMDCFiles(sm.mainPath)
		if err != nil {
			utils.Error("Failed to check for .mdc files | path=" + sm.mainPath + ", error=" + err.Error())
			return fmt.Errorf("failed to check for .mdc files: %v", err)
		}

		if !hasMDCFiles {
			utils.Debug("Main rules location exists but contains no .mdc files | path=" + sm.mainPath)
			ui.Warning("Main rules location exists but contains no rules: %s", sm.mainPath)
			mainLocationNeedsSetup = true
		}
	}

	// If main location doesn't exist or is empty, offer options
	if mainLocationNeedsSetup {
		if !sm.offerMainLocationOptions() {
			ui.Info("Operation cancelled by user")
			return fmt.Errorf("operation cancelled by user")
		}
	}

	// Check if target already exists and list its contents
	if utils.DirExists(targetPath) {
		utils.Debug("Rules directory already exists | path=" + targetPath)

		// List files that will be overwritten
		files, err := utils.ListDirectoryContents(targetPath)
		if err != nil {
			utils.Error("Failed to list directory contents | path=" + targetPath + ", error=" + err.Error())
			return fmt.Errorf("failed to list directory contents: %v", err)
		}

		if len(files) > 0 {
			ui.Header("The following files will be deleted:")
			ui.DisplayFileTable(files)

			ui.Plain("")
			if !ui.PromptYesNo("Do you want to continue and overwrite these files?") {
				ui.Info("Operation cancelled by user")
				return fmt.Errorf("operation cancelled by user")
			}
		} else {
			ui.Info("Destination directory exists but is empty")
		}
	}

	// Copy from main to current
	utils.Debug("Copying rules to current directory | source=" + sm.mainPath + ", target=" + targetPath)
	if err := utils.CopyDir(sm.mainPath, targetPath); err != nil {
		utils.Error("Failed to copy rules | source=" + sm.mainPath + ", target=" + targetPath + ", error=" + err.Error())
		return fmt.Errorf("failed to copy rules: %v", err)
	}

	// Register this project
	utils.Debug("Registering project | project=" + currentDir)
	if err := sm.registry.AddProject(currentDir); err != nil {
		utils.Error("Failed to register project | project=" + currentDir + ", error=" + err.Error())
		return err
	}

	utils.Info("Rules initialized successfully | project=" + currentDir)
	ui.Success("Successfully initialized rules in %s", targetPath)
	return nil
}

// offerMainLocationOptions presents options for an empty or non-existent main location
// Returns true if operation should continue, false if cancelled
func (sm *SyncManager) offerMainLocationOptions() bool {
	options := []string{
		"Create empty directory structure",
		"Fetch from git repository",
		"Cancel operation",
	}

	choice := ui.PromptOptions("Choose an option:", options)

	switch choice {
	case 0: // Create empty directory
		ui.Info("Creating empty directory structure...")

		// Clean up existing directory if it exists
		if utils.DirExists(sm.mainPath) {
			utils.Debug("Existing directory found | path=" + sm.mainPath)
			ui.Warning("Directory already exists: %s", sm.mainPath)

			if !ui.PromptYesNo("Remove existing directory and create empty structure?") {
				ui.Info("Operation cancelled by user")
				return false
			}

			utils.Debug("Removing existing directory | path=" + sm.mainPath)
			if err := os.RemoveAll(sm.mainPath); err != nil {
				utils.Error("Failed to remove existing directory | path=" + sm.mainPath + ", error=" + err.Error())
				ui.Error("Failed to remove existing directory: %v", err)
				return false
			}
		}

		if err := os.MkdirAll(sm.mainPath, sm.config.DirPermission); err != nil {
			utils.Error("Failed to create main directory | path=" + sm.mainPath + ", error=" + err.Error())
			ui.Error("Failed to create main directory: %v", err)
			return false
		}
		ui.Success("Created empty directory structure at %s", sm.mainPath)
		return true

	case 1: // Fetch from git repository
		// Default repository URL
		defaultGitRepo := "git@github.com:nsnarender5511/AgenticSystem.git"
		gitURL := ui.PromptInputWithDefault("Enter git repository URL:", defaultGitRepo, ui.ValidateURL)

		// Clean up existing directory if it exists
		if utils.DirExists(sm.mainPath) {
			utils.Debug("Existing directory found | path=" + sm.mainPath)
			ui.Warning("Directory already exists: %s", sm.mainPath)

			if !ui.PromptYesNo("Remove existing directory before cloning?") {
				ui.Info("Operation cancelled by user")
				return false
			}

			ui.Info("Removing existing directory before cloning...")
			utils.Debug("Removing existing directory | path=" + sm.mainPath)
			if err := os.RemoveAll(sm.mainPath); err != nil {
				utils.Error("Failed to remove existing directory | path=" + sm.mainPath + ", error=" + err.Error())
				ui.Error("Failed to remove existing directory: %v", err)
				return false
			}
		}

		// Verify if the repository exists
		ui.Info("Verifying git repository...")
		if !git.IsValidRepo(gitURL) {
			ui.Error("Invalid git repository URL or repository not accessible")
			return false
		}

		// Clone the repository
		ui.Info("Cloning git repository to %s...", sm.mainPath)
		if err := git.Clone(gitURL, sm.mainPath); err != nil {
			git.CleanupOnFailure(sm.mainPath)
			ui.Error("Failed to clone repository: %v", err)
			return false
		}
		ui.Success("Repository cloned successfully")
		return true

	default: // Cancel
		return false
	}
}

// Merge copies current rules to main and syncs to all locations
func (sm *SyncManager) Merge() error {
	currentDir, err := os.Getwd()
	if err != nil {
		utils.Error("Cannot get current directory | error=" + err.Error())
		return fmt.Errorf("cannot get current directory: %v", err)
	}

	sourcePath := filepath.Join(currentDir, sm.config.RulesDirName)
	utils.Debug("Checking for rules in current directory | path=" + sourcePath)
	if !utils.DirExists(sourcePath) {
		utils.Error("Rules not found in current directory | path=" + sourcePath)
		return fmt.Errorf("%s not found in current directory", sm.config.RulesDirName)
	}

	// Copy to main
	utils.Debug("Copying rules to main location | source=" + sourcePath + ", target=" + sm.mainPath)
	if err := utils.CopyDir(sourcePath, sm.mainPath); err != nil {
		utils.Error("Failed to copy to main | source=" + sourcePath + ", target=" + sm.mainPath + ", error=" + err.Error())
		return fmt.Errorf("failed to copy to main: %v", err)
	}
	utils.Info("Rules merged to main location | source=" + sourcePath)

	// Sync to all registered projects
	utils.Debug("Starting sync to all registered projects")
	return sm.syncToAll()
}

// Sync forces sync from main to current
func (sm *SyncManager) Sync() error {
	currentDir, err := os.Getwd()
	if err != nil {
		utils.Error("Cannot get current directory | error=" + err.Error())
		return fmt.Errorf("cannot get current directory: %v", err)
	}

	targetPath := filepath.Join(currentDir, sm.config.RulesDirName)
	utils.Debug("Syncing rules from main location | source=" + sm.mainPath + ", target=" + targetPath)

	if err := utils.CopyDir(sm.mainPath, targetPath); err != nil {
		utils.Error("Failed to sync rules | source=" + sm.mainPath + ", target=" + targetPath + ", error=" + err.Error())
		return err
	}

	utils.Info("Rules synced successfully | target=" + targetPath)
	return nil
}

// syncToAll syncs main rules to all registered projects
func (sm *SyncManager) syncToAll() error {
	projects := sm.registry.GetProjects()
	utils.Debug("Syncing to all projects | count=" + fmt.Sprintf("%d", len(projects)))

	succeeded := 0
	failed := 0

	for _, project := range projects {
		// Check if project directory exists
		if !utils.DirExists(project) {
			utils.Warn("Project directory does not exist | project=" + project)
			fmt.Printf("Warning: skipping non-existent project: %s\n", project)
			failed++
			continue
		}

		targetPath := filepath.Join(project, sm.config.RulesDirName)
		utils.Debug("Syncing to project | project=" + project + ", target=" + targetPath)

		if err := utils.CopyDir(sm.mainPath, targetPath); err != nil {
			utils.Warn("Failed to sync to project | project=" + project + ", error=" + err.Error())
			fmt.Printf("Warning: failed to sync to %s: %v\n", project, err)
			failed++
		} else {
			succeeded++
		}
	}

	utils.Info("Sync to all projects completed | successful=" + fmt.Sprintf("%d", succeeded) + ", failed=" + fmt.Sprintf("%d", failed))
	return nil
}

// GetRegistry returns the registry instance
func (sm *SyncManager) GetRegistry() *Registry {
	return sm.registry
}

// Clean removes non-existent projects from registry
func (sm *SyncManager) Clean() (int, error) {
	return sm.registry.CleanProjects()
}
