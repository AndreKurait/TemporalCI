<script>
	import { onMount } from 'svelte';
	import { fetchRepos } from '$lib/api.js';
	import { statusIcon, statusColor, formatRelativeTime } from '$lib/ci/status.js';

	let repos = $state([]);
	let loading = $state(true);

	onMount(async () => {
		try {
			const data = await fetchRepos();
			repos = Array.isArray(data) ? data : data.repos || [];
		} catch (e) {
			console.error('Failed to load repos:', e);
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head><title>Repos — TemporalCI</title></svelte:head>

<div class="page">
	<h2>Repositories</h2>

	{#if loading}
		<p class="msg">Loading...</p>
	{:else if !repos.length}
		<p class="msg">No repositories registered yet.</p>
	{:else}
		<div class="grid">
			{#each repos as repo}
				<div class="card repo-card">
					<div class="name">{repo.fullName || repo.name}</div>
					<div class="meta">
						{#if repo.defaultBranchStatus}
							<span style="color: {statusColor(repo.defaultBranchStatus)}">
								{statusIcon(repo.defaultBranchStatus)} {repo.defaultBranch || 'main'}
							</span>
						{/if}
						{#if repo.lastBuildAt}
							<span>Last build: {formatRelativeTime(repo.lastBuildAt)}</span>
						{/if}
						{#if repo.avgDuration}
							<span>Avg: {Math.round(repo.avgDuration / 1000)}s</span>
						{/if}
					</div>
					{#if repo.recentBuilds?.length}
						<div class="sparkline">
							{#each repo.recentBuilds as b}
								<span class="dot" style="background: {statusColor(b.status)}" title={b.status}></span>
							{/each}
						</div>
					{/if}
					<div class="actions">
						<a href="/ci/builds?repo={encodeURIComponent(repo.fullName || repo.name)}">Builds</a>
						<a href="/ci/triggers?repo={encodeURIComponent(repo.fullName || repo.name)}">Trigger</a>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<style>
	.page h2 { margin-bottom: 16px; font-size: 1.25rem; }
	.grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 12px; }
	.repo-card { display: flex; flex-direction: column; gap: 8px; }
	.name { font-weight: 600; font-size: 1rem; }
	.meta { display: flex; gap: 12px; font-size: 0.8rem; color: var(--text-muted); }
	.sparkline { display: flex; gap: 3px; align-items: center; }
	.dot { width: 8px; height: 8px; border-radius: 50%; }
	.actions { display: flex; gap: 8px; font-size: 0.8rem; }
	.msg { color: var(--text-muted); text-align: center; padding: 40px; }
</style>
