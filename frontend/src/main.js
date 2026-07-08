// Diese Bindings werden von "wails build" bzw. "wails dev" automatisch
// unter frontend/wailsjs/ generiert (aus den exportierten Methoden von
// app.go). Vor dem ersten Build existieren sie noch nicht - das ist
// normal, siehe README.md.
import { GetDefaults, StartTelegraf, StopTelegraf, IsRunning } from '../wailsjs/go/main/App.js';
import { EventsOn } from '../wailsjs/runtime/runtime.js';

const $ = (id) => document.getElementById(id);

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

function applyDefaults(cfg) {
  $('deviceUrl').value = cfg.deviceUrl;
  $('interval').value = cfg.interval;
  $('csvEnabled').checked = cfg.csv.enabled;
  $('csvPath').value = cfg.csv.path;
  $('influxUrl').value = cfg.influxdb.url;
  $('influxOrg').value = cfg.influxdb.org;
  $('influxBucket').value = cfg.influxdb.bucket;
  $('pgPort').value = cfg.postgres.port;
  $('pgDatabase').value = cfg.postgres.database;
  $('pgUser').value = cfg.postgres.user;
  $('myPort').value = cfg.mysql.port;
  $('myDatabase').value = cfg.mysql.database;
  $('myUser').value = cfg.mysql.user;
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

window.addEventListener('DOMContentLoaded', async () => {
  const defaults = await GetDefaults();
  applyDefaults(defaults);

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
});
