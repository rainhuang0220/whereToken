const ABSOLUTE_TOKEN_CAP = 1_000_000_000;

export function heatIntensity(value, level, metric = 'tokens') {
  if (metric === 'tokens') return Math.sqrt(Math.min(Math.max(Number(value) || 0, 0) / ABSOLUTE_TOKEN_CAP, 1));
  return level > 0 ? Math.min(level / 5, 1) : 0;
}

export function formatUsageValue(value) {
  return value == null ? '—' : String(value);
}

export function trendPoints(activity) {
  return activity.dates.map((date, index) => ({ date, value: activity.values[index], state: activity.states[index] }))
    .filter((point) => point.state !== 'unknown' && point.state !== 'future' && point.value != null)
    .map(({ date, value }) => ({ date, value }));
}

// The physical front page uses the exact same public sanitized snapshot and
// currently selected metric/range/filter as the accessible HTML dashboard.
// No private API, local usage log or browser credential is ever consulted.
export function projectSnapshot(snapshot, selection = {}) {
  const range = ['all','today','7d','30d','53w'].includes(selection.range) ? selection.range : 'all';
  const metric = selection.metric === 'requests' ? 'requests' : 'tokens';
  const tab = ['agents','providers','models'].includes(selection.tab) ? selection.tab : 'agents';
  const period = snapshot.periods[range] || snapshot.periods.all;
  const rows = tab === 'providers' ? period.by_vendor || [] :
    tab === 'models' ? period.by_model || [] : period.by_agent || [];
  const selected = rows.find(row => row.id === selection.filter);
  const dimension = !selected ? 'all' : tab === 'providers' ? 'vendor' :
    tab === 'models' ? 'model' : 'agent';
  const series = (snapshot.activity.series || []).find(entry =>
    entry.dimension === dimension && (entry.id || 'all') ===
    (selected ? selected.id : 'all') && (entry.metric || 'tokens') === metric);
  const dates = snapshot.activity.dates || [];
  const validIndices = dates.map((date, index) => ({date,index}))
    .filter(item => item.date <= snapshot.activity.to);
  return {
    range, metric, tab, selection: selected ? selected.label : '',
    total: metric === 'requests' ? (selected ? selected.requests : period.requests) :
      (selected ? selected.totals.total : period.totals.total),
    requests: period.requests,
    activeDays: period.active_days,
    streak: period.current_streak,
    asOf: snapshot.as_of_date,
    dataStatus: snapshot.data_status,
    provenance: snapshot.provenance?.kind,
    activity: {
      dates: validIndices.map(({date}) => date),
      values: validIndices.map(({index}) =>
        series && series.values[index] != null ? series.values[index] : null),
      states: validIndices.map(({index}) =>
        series ? (series.states[index] || 'unknown') : 'unknown'),
      intensities: validIndices.map(({index}) =>
        series ? heatIntensity(series.values[index],series.levels[index],metric) : 0),
    },
    breakdown: rows.map(row => ({
      id: row.id, label: row.label,
      total: metric === 'requests' ? row.requests : row.totals.total,
      requests: row.requests, share: row.share
    }))
  };
}
