<script>
	import { onMount } from 'svelte';
	import { fetchRepos, fetchAnalytics } from '$lib/api.js';
	import { formatDuration } from '$lib/ci/status.js';

	let repos = $state([]);
	let selectedRepo = $state('');
	let days = $state(7);
	let analytics = $state(null);
	let loading = $state(false);

	onMount(async () => {
		try {
			const data = await fetchRepos();
			repos = Array.isArray(data) ? data : data.repos || [];
			if (repos.length) {
				selectedRepo = repos[0].fullName || repos[0].name;
				load();
			}
		} catch {}
	});

	async function load() {
		if (!selectedRepo) return;
		loading = true;
		try {
			analytics = await fetchAnalytics(selectedRepo, days);
		} catch (e) {
			console.error('Failed to load analytics:', e);
			analytics = null;
		} finally {
			loading = false;
		}
	}

	let chartPoints = $derived(() => {
		if (!analytics?.durationTrend?.length) return '';
		const data = analytics.durationTrend;
		const maxVal = Math.max(...data.map(d => d.duration));
		const w = 600, h = 120, pad = 4;
		return data.map((d, i) => {
			const x = pad + (i / (data.length - 1)) * (w - pad * 2);
			const y = h - pad - (d.duration / maxVal) * (h - pad * 2);
			return `${x},${y}`;
		}).join(' ');
	});
</script>

<svelte:head><title>Analytics — TemporalCI</title></svelte:head>

<div class="page">
	<h2>Analytics</h2>

	<div class="controls">
		<select bind:value={selectedRepo} onchange={load}>
			{#each repos as r}
				<option value={r.fullName || r.name}>{r.fullName || r.name}</option>
			{/each}
		</select>
		<div class="range">
			{#each [7, 30, 90] as d}
				<button class:active={days === d} onclick={() => { days = d; load(); }}>{d}d</button>
			{/each}
		</div>
	</div>

	{#if loading}
		<p class="msg">Loading...</p>
	{:else if analytics}
		<div class="stats">
			<div class="card stat">
				<div class="label">Success Rate</div>
				<div class="value">{analytics.successRate ?? 0}%</div>
				<div class="bar">
					<div class="fill" style="width: {analytics.successRate ?? 0}%; background: var(--success)"></div>
				</div>
			</div>
			<div class="card stat">
				<div class="label">Avg Duration</div>
				<div class="value">{formatDuration(analytics.avgDuration)}</div>
				{#if analytics.durationTrend?.length}
					<div class="trend">
						{@const last = analytics.durationTrend.at(-1)?.duration}
						{@const prev = analytics.durationTrend.at(-2)?.duration}
						{#if last && prev}
							<span style="color: {last > prev ? 'var(--error)' : 'var(--success)'}">
								{last > prev ? '↑' : '↓'} {Math.abs(Math.round((last - prev) / prev * 100))}%
							</span>
						{/if}
					</div>
				{/if}
			</div>
		</div>

		{#if analytics.durationTrend?.length}
			<div class="card chart-card">
				<div class="label">Duration Trend</div>
				<svg viewBox="0 0 600 120" class="chart">
					<polyline points={chartPoints()} fill="none" stroke="var(--text-link)" stroke-width="2" />
				</svg>
			</div>
		{/if}

		{#if analytics.failingSteps?.length}
			<div class="card">
				<div class="label">Most Failing Steps</div>
				<table>
					<thead><tr><th>Step</th><th>Failures</th><th>Rate</th></tr></thead>
					<tbody>
						{#each analytics.failingSteps as step}
							<tr>
								<td>{step.name}</td>
								<td>{step.failures}</td>
								<td>{step.failRate}%</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}

		{#if analytics.slowestSteps?.length}
			<div class="card">
				<div class="label">Slowest Steps</div>
				<table>
					<thead><tr><th>Step</th><th>Avg Duration</th></tr></thead>
					<tbody>
						{#each analytics.slowestSteps as step}
							<tr>
								<td>{step.name}</td>
								<td>{formatDuration(step.avgDuration)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	{:else}
		<p class="msg">Select a repository to view analytics.</p>
	{/if}
</div>

<style>
	.page h2 { margin-bottom: 16px; font-size: 1.25rem; }
	.controls { display: flex; gap: 8px; margin-bottom: 16px; align-items: center; }
	.range { display: flex; gap: 2px; }
	.range button { font-size: 0.8rem; padding: 4px 10px; }
	.range button.active { background: var(--text-link); color: white; border-color: var(--text-link); }
	.stats { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-bottom: 16px; }
	.stat { display: flex; flex-direction: column; gap: 6px; }
	.label { font-size: 0.8rem; color: var(--text-muted); margin-bottom: 4px; }
	.value { font-size: 1.5rem; font-weight: 700; }
	.bar { height: 6px; background: var(--bg-surface); border-radius: 3px; overflow: hidden; }
	.fill { height: 100%; border-radius: 3px; }
	.trend { font-size: 0.8rem; }
	.chart-card { margin-bottom: 16px; }
	.chart { width: 100%; height: 120px; }
	table { width: 100%; font-size: 0.8rem; border-collapse: collapse; margin-top: 8px; }
	th { text-align: left; color: var(--text-muted); padding: 6px 8px; border-bottom: 1px solid var(--border); }
	td { padding: 6px 8px; border-bottom: 1px solid var(--border); }
	.card + .card { margin-top: 12px; }
	.msg { color: var(--text-muted); text-align: center; padding: 40px; }
</style>
