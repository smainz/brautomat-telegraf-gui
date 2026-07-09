// Diese Bindings werden von "wails build" bzw. "wails dev" automatisch
// unter frontend/wailsjs/ generiert (aus den exportierten Methoden von
// app.go). Vor dem ersten Build existieren sie noch nicht - das ist
// normal, siehe README.md.
import {
  GetDefaults,
  StartTelegraf,
  StopTelegraf,
  IsRunning,
  SaveConfig,
  LoadConfig,
  ChooseSaveConfigPath,
  ChooseOpenConfigPath,
  GetDefaultConfigPath,
} from '../wailsjs/go/main/App.js';
import { EventsOn } from '../wailsjs/runtime/runtime.js';

const $ = (id) => document.getElementById(id);

// '' bedeutet: Standardpfad (~/.brautomat-telegraf-gui/config.json).
// Wird beim Laden/Speichern unter... aktualisiert, damit ein späteres
// "Speichern" ohne erneuten Dialog denselben Pfad wieder verwendet.
let currentConfigPath = '';

function collectConfig() {
  return {
    deviceUrl: $('deviceUrl').value,
    interval: $('interval').value || '30s',
    csv: {
      enabled: $('csvEnabled').checked,
      path: $('csvPath').value,
    },
    influxdb: {
      enabled: $('influxEnabled').checked,
      url: $('influxUrl').value,
      token: $('influxToken').value,
      org: $('influxOrg').value,
      bucket: $('influxBucket').value,
    },
    postgres: {
      enabled: $('pgEnabled').checked,
      host: $('pgHost').value,
      port: $('pgPort').value,
      database: $('pgDatabase').value,
      user: $('pgUser').value,
      password: $('pgPassword').value,
    },
    mysql: {
      enabled: $('myEnabled').checked,
      host: $('myHost').value,
      port: $('myPort').value,
      database: $('myDatabase').value,
      user: $('myUser').value,
      password: $('myPassword').value,
    },
  };
}

// Befüllt das komplette Formular aus einem Config-Objekt - egal ob das
// von GetDefaults(), LoadConfig() oder einer manuell geöffneten Datei
// stammt. Deckt bewusst auch die Enabled-Checkboxen und Secret-Felder ab,
// die beim reinen Vorbelegen mit Defaults zuvor nicht gesetzt wurden.
function applyConfig(cfg) {
  $('deviceUrl').value = cfg.deviceUrl ?? '';
  $('interval').value = cfg.interval ?? '30s';

  $('csvEnabled').checked = !!cfg.csv?.enabled;
  $('csvPath').value = cfg.csv?.path ?? '';

  $('influxEnabled').checked = !!cfg.influxdb?.enabled;
  $('influxUrl').value = cfg.influxdb?.url ?? '';
  $('influxToken').value = cfg.influxdb?.token ?? '';
  $('influxOrg').value = cfg.influxdb?.org ?? '';
  $('influxBucket').value = cfg.influxdb?.bucket ?? '';

  $('pgEnabled').checked = !!cfg.postgres?.enabled;
  $('pgHost').value = cfg.postgres?.host ?? '';
  $('pgPort').value = cfg.postgres?.port ?? '';
  $('pgDatabase').value = cfg.postgres?.database ?? '';
  $('pgUser').value = cfg.postgres?.user ?? '';
  $('pgPassword').value = cfg.postgres?.password ?? '';

  $('myEnabled').checked = !!cfg.mysql?.enabled;
  $('myHost').value = cfg.mysql?.host ?? '';
  $('myPort').value = cfg.mysql?.port ?? '';
  $('myDatabase').value = cfg.mysql?.database ?? '';
  $('myUser').value = cfg.mysql?.user ?? '';
  $('myPassword').value = cfg.mysql?.password ?? '';

  // Checkboxen wurden hier programmatisch gesetzt, was kein "change"-Event
  // auslöst. tabs.js hört auf dieses Event, um die Tab-Indikatoren
  // (welche Ziele sind aktiviert) neu zu synchronisieren.
  window.dispatchEvent(new Event('brautomat:config-applied'));
}

function appendLog(line) {
  const log = $('log');
  log.textContent += line + '\n';
  log.scrollTop = log.scrollHeight;
}

function setRunning(running) {
  $('startBtn').disabled = running;
  $('stopBtn').disabled = !running;
  $('status').textContent = running ? 'läuft' : 'gestoppt';
}

function showConfigPath(path) {
  currentConfigPath = path;
  $('configPath').textContent = path;
}

// Lädt beim Start die zuletzt gespeicherte Konfiguration vom
// Standardpfad. Existiert dort noch keine Datei, liefert LoadConfig('')
// bereits die Default()-Werte zurück (siehe app.go), sodass hier kein
// Sonderfall nötig ist.
async function loadInitialConfig() {
  try {
    const cfg = await LoadConfig('');
    applyConfig(cfg);
    const defaultPath = await GetDefaultConfigPath();
    showConfigPath(defaultPath);
  } catch (err) {
    appendLog('[Fehler beim Laden der Konfiguration] ' + err);
    applyConfig(await GetDefaults());
  }
}

window.addEventListener('DOMContentLoaded', async () => {
  await loadInitialConfig();

  const running = await IsRunning();
  setRunning(running);

  EventsOn('telegraf:log', appendLog);
  EventsOn('telegraf:status', (status) => setRunning(status === 'running'));

  $('startBtn').addEventListener('click', async () => {
    try {
      await StartTelegraf(collectConfig());
    } catch (err) {
      appendLog('[Fehler] ' + err);
    }
  });

  $('stopBtn').addEventListener('click', async () => {
    try {
      await StopTelegraf();
    } catch (err) {
      appendLog('[Fehler] ' + err);
    }
  });

  // Speichert unter dem zuletzt verwendeten Pfad (Standardpfad, falls
  // noch nie ein eigener Pfad gewählt wurde) - ohne erneuten Dialog.
  $('saveBtn').addEventListener('click', async () => {
    try {
      const savedPath = await SaveConfig(collectConfig(), currentConfigPath);
      showConfigPath(savedPath);
      appendLog('[Config] gespeichert unter ' + savedPath);
    } catch (err) {
      appendLog('[Fehler beim Speichern] ' + err);
    }
  });

  // Öffnet immer den nativen "Speichern unter"-Dialog, damit der
  // Benutzer einen beliebigen Pfad wählen kann.
  $('saveAsBtn').addEventListener('click', async () => {
    try {
      const chosen = await ChooseSaveConfigPath();
      if (!chosen) return; // Dialog abgebrochen
      const savedPath = await SaveConfig(collectConfig(), chosen);
      showConfigPath(savedPath);
      appendLog('[Config] gespeichert unter ' + savedPath);
    } catch (err) {
      appendLog('[Fehler beim Speichern] ' + err);
    }
  });

  $('loadBtn').addEventListener('click', async () => {
    try {
      const chosen = await ChooseOpenConfigPath();
      if (!chosen) return; // Dialog abgebrochen
      const cfg = await LoadConfig(chosen);
      applyConfig(cfg);
      showConfigPath(chosen);
      appendLog('[Config] geladen aus ' + chosen);
    } catch (err) {
      appendLog('[Fehler beim Laden] ' + err);
    }
  });
});
