// Reine UI-Logik für die Tab-Navigation im "Ziele"-Bereich. Bewusst
// getrennt von main.js (das die eigentliche Formular-/Start-Stop-Logik
// enthält), damit beide unabhängig voneinander bleiben.
//
// Wichtig: Tabs sind nur eine Ansichts-Umschaltung. Die "Ziel
// aktivieren"-Checkbox jedes Tabs bleibt unabhängig davon, welcher Tab
// gerade sichtbar ist - mehrere Ziele können also weiterhin gleichzeitig
// aktiviert sein, auch wenn man sie nacheinander in unterschiedlichen
// Tabs anhakt.

const tabButtons = document.querySelectorAll('.tab-btn');
const tabPanels = document.querySelectorAll('.tab-panel');

function activateTab(tabId) {
  tabButtons.forEach((btn) => {
    const isActive = btn.dataset.tab === tabId;
    btn.classList.toggle('active', isActive);
    btn.setAttribute('aria-selected', String(isActive));
  });
  tabPanels.forEach((panel) => {
    panel.classList.toggle('active', panel.dataset.tabPanel === tabId);
  });
}

tabButtons.forEach((btn) => {
  btn.addEventListener('click', () => activateTab(btn.dataset.tab));
});

// Markiert den Tab-Button visuell (kleiner Punkt), wenn das zugehörige
// Ziel aktiviert ist - so sieht man auch ohne Tabwechsel, welche Ziele
// gerade eingeschaltet sind.
const enabledCheckboxIdByTab = {
  csv: 'csvEnabled',
  influx: 'influxEnabled',
  pg: 'pgEnabled',
  my: 'myEnabled',
};

function syncEnabledIndicator(tabId) {
  const checkbox = document.getElementById(enabledCheckboxIdByTab[tabId]);
  const btn = document.querySelector(`.tab-btn[data-tab="${tabId}"]`);
  if (checkbox && btn) {
    btn.classList.toggle('target-enabled', checkbox.checked);
  }
}

function syncAllEnabledIndicators() {
  Object.keys(enabledCheckboxIdByTab).forEach(syncEnabledIndicator);
}

Object.keys(enabledCheckboxIdByTab).forEach((tabId) => {
  const checkbox = document.getElementById(enabledCheckboxIdByTab[tabId]);
  if (checkbox) {
    checkbox.addEventListener('change', () => syncEnabledIndicator(tabId));
  }
});

syncAllEnabledIndicators();

// main.js befüllt die Checkboxen programmatisch (GetDefaults/LoadConfig),
// was keine "change"-Events auslöst. main.js meldet das per
// CustomEvent, damit die Tab-Indikatoren danach erneut synchronisiert
// werden.
window.addEventListener('brautomat:config-applied', syncAllEnabledIndicators);
