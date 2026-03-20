<script>
	import { page } from '$app/stores';
	import { onMount, onDestroy } from 'svelte';
	import { fetchBuild, rerunBuild } from '$lib/api.js';
	import { statusIcon, statusColor, formatDuration, formatRelativeTime } from '$lib/ci/status.js';
	import StepList from '$lib/ci/StepList.svelte';
	import DagView from '$lib/ci/DagView.svelte';

	let build = $state(null);
	let loading = $state(true);
	let tab = $state('steps');
	let timer;

	let workflowId = $derived($page.params.workflowId);
	let buildNumber = $derived(workflowId?.split('-').pop() || workflowId);
	let isRunning = $derived(build?.status === 'running' || build?.status === 'in_progress');

	async function load() {
		try {
			build = await fetchBuild(workflowId);
		} catch (e) {
			console.error('Failed to load build:', e);
		} finally {
			loading = false;
		}
	}

	async function handleRerun() {
		try {
			const result = await rerunBuild(workflowId);
			if (result?.workflowId) {
				window.location.href = `/ci/builds/${result.workflowId}`;
			} else {
				load();
			}
		} catch (e) {
			alert('Re-run failed: ' + e.message);
		}
	}

	function scrollToStep(name) {
		tab = 'steps';
	}

	onMount(() => {
		load();
		timer = setInterval(() => { if (isRunning) load(); }, 5000);
	});

	onDestroy(() => clearInterval(timer));
</script>

<svelte:head><title>Build #{buildNumber} — TemporalCI</title></svelte:head>

<div class="page">
	<a href="/ci/builds" class="back">← Builds</a>

	{#if loading}
		<p class="msg">Loading...</p>
	{:else if build}
		<div class="header">
			<div class="title">
				<span class="status" style="color: {statusColor(build.status)}">{statusIcon(build.status)}</span>
				<span class="repo">{build.repo || build.owner + '/' + build.repoName}</span>
				<span class="num">#{buildNumber}</span>
			</div>
			<div class="meta">
				{#if build.branch}<span class="branch">{build.branch}</span>{/if}
				{#if build.trigger}<span>Triggered by {build.trigger}</span>{/if}
				{#if build.duration}<span>{formatDuration(build.duration)}</span>{/if}
				{#if build.startedAt}<span>{formatRelativeTime(build.startedAt)}</span>{/if}
			</div>
			<div class="actions">
				{#if build.htmlUrl}
					<a href={build.htmlUrl} target="_blank" rel="noopener">View on GitHub ↗</a>
				{/if}
				<a href="/workflows/{workflowId}" target="_blank" rel="noopener">View Workflow ↗</a>
				<button onclick={handleRerun}>Re-run</button>
			</div>
		</div>

		<div class="tabs">
			<button class:active={tab === 'steps'} onclick={() => tab = 'steps'}>Steps</button>
			<button class:active={tab === 'dag'} onclick={() => tab = 'dag'}>Pipeline</button>
		</div>

		{#if tab === 'steps'}
			<StepList steps={build.steps || []} {workflowId} />
		{:else}
			<DagView steps={build.steps || []} onNodeClick={scrollToStep} />
		{/if}
	{:else}
		<p class="msg">Build not found</p>
	{/if}
</div>

<style>
	.back { display: inline-block; margin-bottom: 12px; font-size: 0.875rem; }
	.header { margin-bottom: 16px; }
	.title { display: flex; align-items: center; gap: 8px; font-size: 1.25rem; }
	.repo { font-weight: 600; }
	.num { color: var(--text-muted); }
	.meta { display: flex; gap: 12px; margin-top: 6px; font-size: 0.8rem; color: var(--text-muted); }
	.branch {
		background: var(--bg-surface);
		padding: 1px 6px;
		border-radius: 3px;
		font-family: var(--font-mono);
		font-size: 0.75rem;
	}
	.actions { display: flex; gap: 8px; margin-top: 10px; align-items: center; }
	.actions a {
		font-size: 0.8rem;
		padding: 4px 10px;
		border: 1px solid var(--border);
		border-radius: var(--radius);
	}
	.tabs { display: flex; gap: 2px; margin-bottom: 16px; }
	.tabs button { padding: 6px 16px; }
	.tabs button.active { background: var(--text-link); color: white; border-color: var(--text-link); }
	.msg { color: var(--text-muted); text-align: center; padding: 40px; }
</style>
