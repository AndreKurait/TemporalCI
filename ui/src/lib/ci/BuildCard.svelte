<script>
	import { statusIcon, statusColor, formatDuration, formatRelativeTime } from './status.js';

	let { build } = $props();

	let buildNumber = $derived(build.workflowId?.split('-').pop() || build.workflowId);
	let stepSummary = $derived(() => {
		if (!build.steps?.length) return '';
		const passed = build.steps.filter(s => s.status === 'success').length;
		const failed = build.steps.filter(s => s.status === 'failed').length;
		return `${passed}/${build.steps.length} passed${failed ? `, ${failed} failed` : ''}`;
	});
</script>

<a href="/ci/builds/{build.workflowId}" class="build-card card">
	<div class="header">
		<span class="status" style="color: {statusColor(build.status)}">{statusIcon(build.status)}</span>
		<span class="repo">{build.repo || build.owner + '/' + build.repoName}</span>
		<span class="number">#{buildNumber}</span>
		<span class="spacer"></span>
		<span class="time">{formatRelativeTime(build.startedAt || build.createdAt)}</span>
	</div>
	<div class="meta">
		{#if build.branch}<span class="branch">{build.branch}</span>{/if}
		{#if build.duration}<span class="duration">{formatDuration(build.duration)}</span>{/if}
	</div>
	{#if build.steps?.length}
		<div class="summary">{stepSummary()}</div>
	{/if}
</a>

<style>
	.build-card {
		display: block;
		text-decoration: none;
		color: var(--text);
		margin-bottom: 8px;
		transition: background 0.15s;
	}
	.build-card:hover { background: var(--bg-hover); text-decoration: none; }
	.header { display: flex; align-items: center; gap: 8px; }
	.repo { font-weight: 600; }
	.number { color: var(--text-muted); font-size: 0.875rem; }
	.spacer { flex: 1; }
	.time { color: var(--text-muted); font-size: 0.8rem; }
	.meta { display: flex; gap: 12px; margin-top: 4px; font-size: 0.8rem; color: var(--text-muted); }
	.branch {
		background: var(--bg-surface);
		padding: 1px 6px;
		border-radius: 3px;
		font-family: var(--font-mono);
		font-size: 0.75rem;
	}
	.summary { font-size: 0.8rem; color: var(--text-muted); margin-top: 4px; }
</style>
