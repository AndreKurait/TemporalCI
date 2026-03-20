<script>
	import { onMount, onDestroy } from 'svelte';
	import { fetchBuilds, fetchRepos } from '$lib/api.js';
	import BuildCard from '$lib/ci/BuildCard.svelte';

	let builds = $state([]);
	let repos = $state([]);
	let repo = $state('');
	let branch = $state('');
	let status = $state('');
	let loading = $state(true);
	let page = $state(1);
	let hasMore = $state(false);
	let timer;

	const PAGE_SIZE = 50;

	async function load(append = false) {
		try {
			const filters = { limit: PAGE_SIZE, offset: append ? builds.length : 0 };
			if (repo) filters.repo = repo;
			if (branch) filters.branch = branch;
			if (status) filters.status = status;
			const data = await fetchBuilds(filters);
			const list = Array.isArray(data) ? data : data.builds || [];
			builds = append ? [...builds, ...list] : list;
			hasMore = list.length === PAGE_SIZE;
		} catch (e) {
			console.error('Failed to load builds:', e);
		} finally {
			loading = false;
		}
	}

	function refresh() {
		const hasRunning = builds.some(b => b.status === 'running' || b.status === 'in_progress');
		if (hasRunning) load();
	}

	onMount(() => {
		load();
		fetchRepos().then(r => repos = Array.isArray(r) ? r : r.repos || []).catch(() => {});
		timer = setInterval(refresh, 5000);
	});

	onDestroy(() => clearInterval(timer));
</script>

<svelte:head><title>Builds — TemporalCI</title></svelte:head>

<div class="page">
	<h2>Builds</h2>
	<div class="filters">
		<select bind:value={repo} onchange={() => load()}>
			<option value="">All repos</option>
			{#each repos as r}
				<option value={r.fullName || r.name}>{r.fullName || r.name}</option>
			{/each}
		</select>
		<input type="text" placeholder="Branch..." bind:value={branch} onchange={() => load()} />
		<div class="status-toggles">
			{#each ['', 'passed', 'failed', 'running'] as s}
				<button class:active={status === s} onclick={() => { status = s; load(); }}>
					{s || 'All'}
				</button>
			{/each}
		</div>
	</div>

	{#if loading}
		<p class="msg">Loading...</p>
	{:else if !builds.length}
		<p class="msg">No builds found. Push a commit or trigger a build to get started.</p>
	{:else}
		{#each builds as build (build.workflowId)}
			<BuildCard {build} />
		{/each}
		{#if hasMore}
			<button class="load-more" onclick={() => load(true)}>Load more</button>
		{/if}
	{/if}
</div>

<style>
	.page h2 { margin-bottom: 16px; font-size: 1.25rem; }
	.filters { display: flex; gap: 8px; margin-bottom: 16px; flex-wrap: wrap; align-items: center; }
	.status-toggles { display: flex; gap: 2px; }
	.status-toggles button { font-size: 0.8rem; padding: 4px 10px; }
	.status-toggles button.active { background: var(--text-link); color: white; border-color: var(--text-link); }
	.msg { color: var(--text-muted); text-align: center; padding: 40px; }
	.load-more { display: block; margin: 16px auto; }
</style>
