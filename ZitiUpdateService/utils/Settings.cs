﻿using System;
using System.IO;
using Newtonsoft.Json;
using NLog;
using ZitiDesktopEdge.DataStructures;

namespace ZitiUpdateService.Utils {
	internal class Settings {
		private static readonly Logger Logger = LogManager.GetCurrentClassLogger();
		private FileSystemWatcher watcher;

		[JsonIgnore]
		private string Location { get; set; }

		public bool AutomaticUpdatesDisabled { get; set; }
		public string AutomaticUpdateURL { get; set; }

		public event System.EventHandler<ControllerEvent> OnConfigurationChange;

		internal Settings(bool doInit) {
			if (doInit) {
				init();
			}
		}

		public Settings() {
		}

		private void init() {
			string folder = Path.Combine(System.Environment.GetFolderPath(System.Environment.SpecialFolder.ApplicationData), "NetFoundry", "ZitiUpdateService");
			string file = "settings.json";
			Location = Path.Combine(folder, file);
			Directory.CreateDirectory(folder);
			watcher = new FileSystemWatcher(folder);
			watcher.Filter = file;

			watcher.NotifyFilter = NotifyFilters.Attributes
								 | NotifyFilters.CreationTime
								 | NotifyFilters.DirectoryName
								 | NotifyFilters.FileName
								 | NotifyFilters.LastAccess
								 | NotifyFilters.LastWrite
								 | NotifyFilters.Security
								 | NotifyFilters.Size;
			watcher.Changed += OnChanged;
			watcher.Deleted += OnDeleted;
			watcher.Renamed += OnRenamed;
			watcher.Error += OnError;
			watcher.EnableRaisingEvents = true;
		}

		private static JsonSerializer serializer = new JsonSerializer() { Formatting = Formatting.Indented };
		internal void Load() {
			try {
				string json = File.ReadAllText(Location);
				var jsonReaderEvt = new JsonTextReader(new StringReader(json));
				Settings s = serializer.Deserialize<Settings>(jsonReaderEvt);
				if (s != null) {
					Update(s);
				} else {
					Logger.Debug("settings file was null? file doesn't exist or file was garbage?");
				}
			} catch (Exception ex) {
				// do nothing, probably means the file is just or doesn't exist etc.
				Logger.Debug("unexpected error loading settings. file was null? file doesn't exist or file was garbage? {0}", ex);
			}
		}
		internal void Write() {
			lock (this) {
				this.watcher.Changed -= OnChanged;
				try {
					using (StreamWriter file = File.CreateText(Location)) {
						serializer.Serialize(file, this);
						file.Flush();
						file.Close();
					}
					this.OnConfigurationChange?.Invoke(null, null);
				} catch {
					// do nothing
				}
				this.watcher.Changed += OnChanged;
			}
		}


		private static void OnError(object sender, ErrorEventArgs e) {
		}

		private void OnRenamed(object sender, RenamedEventArgs e) {
			Logger.Info("Settings file renamed. Resetting to defaults...");
			this.Update(new Settings());
		}

		private void OnDeleted(object sender, FileSystemEventArgs e) {
			Logger.Info("Settings file deleted. Resetting to defaults...");
			this.Update(new Settings());
		}

		private void OnCreated(object sender, FileSystemEventArgs e) {
		}

		private void OnChanged(object sender, FileSystemEventArgs e) {
			Logger.Info("Settings file changed. Reloading...");
			this.Load();
			this.OnConfigurationChange?.Invoke(null, null);
		}

		private void Update(Settings source) {
			this.AutomaticUpdatesDisabled = source.AutomaticUpdatesDisabled;
			this.AutomaticUpdateURL = source.AutomaticUpdateURL;
		}
	}
}
