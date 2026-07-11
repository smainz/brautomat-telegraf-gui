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
  ChooseTemplatesDir,
  ChooseExportTemplatesDir,
  ExportTemplates,
  ChooseSaveLogPath,
  SaveLog,
  TestDeviceConnection,
  ChooseTelegrafPath,
  ChooseTelegrafDownloadDir,
  DownloadTelegraf,
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
    // Ist "Eigene Templates verwenden" nicht angehakt, wird bewusst ein
    // leerer String gesendet - das bedeutet für das Backend "interne
    // (eingebettete) Templates verwenden", unabhängig vom zuletzt
    // eingegebenen Pfad im (dann deaktivierten) Textfeld.
    templatesDir: $('customTemplatesEnabled').checked ? $('templatesDir').value : '',
    telegrafPath: $('telegrafPath').value,
    savePasswords: $('savePasswordsEnabled').checked,
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
    mqtt: {
      enabled: $('mqttEnabled').checked,
      server: $('mqttServer').value,
      topic: $('mqttTopic').value,
      clientId: $('mqttClientId').value,
      qos: $('mqttQos').value,
      username: $('mqttUsername').value,
      password: $('mqttPassword').value,
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

  const templatesDir = cfg.templatesDir ?? '';
  $('customTemplatesEnabled').checked = templatesDir !== '';
  $('templatesDir').value = templatesDir;
  syncTemplatesDirState();

  $('telegrafPath').value = cfg.telegrafPath ?? '';

  // Default unchecked, falls nicht in cfg vorhanden (z.B. sehr alte,
  // vor dieser Funktion gespeicherte config.json).
  $('savePasswordsEnabled').checked = !!cfg.savePasswords;

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

  $('mqttEnabled').checked = !!cfg.mqtt?.enabled;
  $('mqttServer').value = cfg.mqtt?.server ?? '';
  $('mqttTopic').value = cfg.mqtt?.topic ?? '';
  $('mqttClientId').value = cfg.mqtt?.clientId ?? '';
  // Fallback '0', falls aus einer älteren config.json ohne MQTT-Feld
  // geladen wird - qos darf im Template nie leer sein (siehe MQTTTarget).
  $('mqttQos').value = cfg.mqtt?.qos || '0';
  $('mqttUsername').value = cfg.mqtt?.username ?? '';
  $('mqttPassword').value = cfg.mqtt?.password ?? '';

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

// Einfaches Pop-up für Fehlermeldungen, z.B. beim fehlgeschlagenen
// Verbindungstest. Schließen per Button, Klick auf den abgedunkelten
// Hintergrund oder Escape.
function showErrorModal(message) {
  $('errorModalMessage').textContent = message;
  $('errorModalOverlay').classList.remove('hidden');
}

function hideErrorModal() {
  $('errorModalOverlay').classList.add('hidden');
}

// Fortschrittsfenster für "telegraf herunterladen…". Zeigt einen
// Status-Text (Zwischenzustände wie "Lade herunter…"/"Entpacke…") und
// einen Fortschrittsbalken - solange die Gesamtgröße unbekannt ist
// (Server ohne Content-Length, oder während des Entpackens), läuft der
// Balken unbestimmt ("indeterminate"); sobald Byte-Zahlen vorliegen,
// zeigt er den tatsächlichen Prozentwert.
function showDownloadModal() {
  $('downloadModalStatus').textContent = 'Vorbereiten…';
  $('downloadModalPercent').textContent = '';
  $('downloadModalProgressFill').style.width = '';
  $('downloadModalProgressFill').classList.add('indeterminate');
  $('downloadModalOverlay').classList.remove('hidden');
}

function hideDownloadModal() {
  $('downloadModalOverlay').classList.add('hidden');
}

function updateDownloadStatus(message) {
  $('downloadModalStatus').textContent = message;
}

function updateDownloadProgress({ downloaded, total }) {
  const fill = $('downloadModalProgressFill');
  const percentEl = $('downloadModalPercent');

  if (total && total > 0) {
    fill.classList.remove('indeterminate');
    const pct = Math.min(100, Math.round((downloaded / total) * 100));
    fill.style.width = pct + '%';
    percentEl.textContent = `${formatBytes(downloaded)} / ${formatBytes(total)} (${pct}%)`;
  } else {
    // Gesamtgröße unbekannt - nur die bisher heruntergeladene Menge
    // anzeigen, Balken bleibt unbestimmt.
    fill.classList.add('indeterminate');
    percentEl.textContent = formatBytes(downloaded) + ' heruntergeladen';
  }
}

function formatBytes(bytes) {
  if (bytes < 1024) return bytes + ' B';
  const units = ['KB', 'MB', 'GB'];
  let value = bytes;
  let unitIndex = -1;
  do {
    value /= 1024;
    unitIndex++;
  } while (value >= 1024 && unitIndex < units.length - 1);
  return value.toFixed(1) + ' ' + units[unitIndex];
}

// Blendet den Bereich mit Pfad-Textfeld, "Durchsuchen…" und
// "Templates exportieren…" komplett aus, solange "Eigene Templates
// verwenden" nicht angehakt ist - nicht nur deaktiviert, sondern
// unsichtbar (siehe .hidden in style.css).
function syncTemplatesDirState() {
  const enabled = $('customTemplatesEnabled').checked;
  $('customTemplatesControls').classList.toggle('hidden', !enabled);
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

  $('errorModalCloseBtn').addEventListener('click', hideErrorModal);
  $('errorModalOverlay').addEventListener('click', (event) => {
    if (event.target === $('errorModalOverlay')) hideErrorModal();
  });
  window.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') {
      hideErrorModal();
      hideDownloadModal();
    }
  });

  $('downloadModalCloseBtn').addEventListener('click', hideDownloadModal);

  EventsOn('telegraf-download:status', updateDownloadStatus);
  EventsOn('telegraf-download:progress', updateDownloadProgress);

  $('testDeviceBtn').addEventListener('click', async () => {
    const result = $('testDeviceResult');
    result.textContent = 'Teste Verbindung…';
    result.classList.remove('test-success');
    $('testDeviceBtn').disabled = true;
    try {
      await TestDeviceConnection($('deviceUrl').value);
      result.textContent = '✓ Verbindung erfolgreich - Telemetrie-Endpunkt antwortet.';
      result.classList.add('test-success');
    } catch (err) {
      result.textContent = '';
      showErrorModal(String(err));
    } finally {
      $('testDeviceBtn').disabled = false;
    }
  });

  $('customTemplatesEnabled').addEventListener('change', syncTemplatesDirState);

  $('browseTelegrafBtn').addEventListener('click', async () => {
    try {
      const chosen = await ChooseTelegrafPath();
      if (!chosen) return; // Dialog abgebrochen
      $('telegrafPath').value = chosen;
    } catch (err) {
      appendLog('[Fehler bei der Dateiauswahl] ' + err);
    }
  });

  $('downloadTelegrafBtn').addEventListener('click', async () => {
    let destDir;
    try {
      destDir = await ChooseTelegrafDownloadDir();
      if (!destDir) return; // Dialog abgebrochen
    } catch (err) {
      appendLog('[Fehler bei der Verzeichnisauswahl] ' + err);
      return;
    }

    const btn = $('downloadTelegrafBtn');
    btn.disabled = true;
    showDownloadModal();
    appendLog('[telegraf] Download nach ' + destDir + ' gestartet…');
    try {
      const path = await DownloadTelegraf(destDir);
      $('telegrafPath').value = path;
      updateDownloadStatus('Fertig: ' + path);
      appendLog('[telegraf] heruntergeladen und entpackt: ' + path);
    } catch (err) {
      updateDownloadStatus('Fehler: ' + err);
      appendLog('[Fehler beim Herunterladen von telegraf] ' + err);
    } finally {
      btn.disabled = false;
    }
  });

  $('browseTemplatesBtn').addEventListener('click', async () => {
    try {
      const chosen = await ChooseTemplatesDir();
      if (!chosen) return; // Dialog abgebrochen
      $('templatesDir').value = chosen;
    } catch (err) {
      appendLog('[Fehler bei der Verzeichnisauswahl] ' + err);
    }
  });

  $('exportTemplatesBtn').addEventListener('click', async () => {
    try {
      const chosen = await ChooseExportTemplatesDir();
      if (!chosen) return; // Dialog abgebrochen
      await ExportTemplates(chosen);
      appendLog('[Templates] exportiert nach ' + chosen);
    } catch (err) {
      appendLog('[Fehler beim Exportieren der Templates] ' + err);
    }
  });

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
      const cfg = collectConfig();
      const savedPath = await SaveConfig(cfg, currentConfigPath);
      showConfigPath(savedPath);
      appendLog('[Config] gespeichert unter ' + savedPath + (cfg.savePasswords ? '' : ' (ohne Passwörter)'));
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
      const cfg = collectConfig();
      const savedPath = await SaveConfig(cfg, chosen);
      showConfigPath(savedPath);
      appendLog('[Config] gespeichert unter ' + savedPath + (cfg.savePasswords ? '' : ' (ohne Passwörter)'));
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

  $('clearLogBtn').addEventListener('click', () => {
    $('log').textContent = '';
  });

  $('saveLogBtn').addEventListener('click', async () => {
    try {
      const chosen = await ChooseSaveLogPath();
      if (!chosen) return; // Dialog abgebrochen
      await SaveLog($('log').textContent, chosen);
      appendLog('[Log] gespeichert unter ' + chosen);
    } catch (err) {
      appendLog('[Fehler beim Speichern der Ausgabe] ' + err);
    }
  });
});
