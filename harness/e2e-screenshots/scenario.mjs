const SCENARIO_PRESETS = {
  shared: {
    requestTitle: "Carrier dispute triage package",
    requestBudgetCents: 285000,
    quoteCents: 241000,
  },
  desktop: {
    requestTitle: "Warehouse relaunch pricing sprint",
    requestBudgetCents: 184000,
    quoteCents: 149000,
  },
  mobile: {
    requestTitle: "Returns backlog stabilization package",
    requestBudgetCents: 126000,
    quoteCents: 98000,
  },
};

export function buildScenarioPreset(seedKey) {
  const preset = SCENARIO_PRESETS[seedKey] ?? SCENARIO_PRESETS.shared;

  return {
    ...preset,
    requestBudgetDollars: formatInputDollars(preset.requestBudgetCents),
    quoteDollars: formatInputDollars(preset.quoteCents),
  };
}

function formatInputDollars(cents) {
  return (cents / 100).toFixed(2);
}
