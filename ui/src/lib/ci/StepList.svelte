<script>
	import { statusIcon, statusColor, formatDuration } from './status.js';
	import LogViewer from './LogViewer.svelte';

	let { steps = [], workflowId = '' } = $props();
	let expanded = $state(new Set());

	// Auto-expand failed steps
	$effect(() => {
		const failed = new Set();
		for (const s of steps) {
			if (s.status === 'failed') failed.add(s.id || s.name);
		}
		expanded = failed;
	});

	function toggle(id) {
		const next = new Set(expanded);
		if (next.has(id)) next.delete(id);
		else next.add(id);
		expanded = next;
	}

	function isMatrix(step) {
		return step.matrix && step.matrix.length > 0;
	}

	function matrixSummary(step) {
		const passed = step.matrix.filter(m => m.status === 'success').length;
		return `${passed}/${step.matrix.length} passed`;
	}
</script>

<div class="step-list">
	{#each steps as step}
		{@const id = step.id || step.name}
		<div class="step" class:failed={step.status === 'failed'}>
			<button class="step-header" onclick={() => toggle(id)}>
				<span class="icon" style="color: {statusColor(step.status)}">{statusIcon(step.status)}</span>
				<span class="name">{step.name}</span>
				{#if isMatrix(step)}
					<span class="badge">{matrixSummary(step)}</span>
				{/if}
				<span class="spacer"></span>
				{#if step.duration}<span class="dur">{formatDuration(step.duration)}</span>{/if}
				<span class="chevron">{expanded.has(id) ? '▾' : '▸'}</span>
			</button>

			{#if expanded.has(id)}
				<div class="step-body">
					{#if isMatrix(step)}
						<div class="matrix">
							{#each step.matrix as m}
								<div class="matrix-item">
									<span style="color: {statusColor(m.status)}">{statusIcon(m.status)}</span>
									<span>{m.name || JSON.stringify(m.values)}</span>
									{#if m.duration}<span class="dur">{formatDuration(m.duration)}</span>{/if}
								</div>
							{/each}
						</div>
					{/if}
					<LogViewer log={step.log || ''} {workflowId} stepId={id} />
				</div>
			{/if}
		</div>
	{/each}
	{#if !steps.length}
		<p class="empty">No steps</p>
	{/if}
</div>

<style>
	.step-list { display: flex; flex-direction: column; gap: 2px; }
	.step {
		border: 1px solid var(--border);
		border-radius: var(--radius);
		overflow: hidden;
	}
	.step.failed { border-color: var(--error); }
	.step-header {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		padding: 10px 12px;
		background: var(--bg-card);
		border: none;
		text-align: left;
		font-size: 0.875rem;
	}
	.step-header:hover { background: var(--bg-hover); }
	.name { font-weight: 500; }
	.badge {
		font-size: 0.75rem;
		background: var(--bg-surface);
		padding: 1px 6px;
		border-radius: 3px;
		color: var(--text-muted);
	}
	.spacer { flex: 1; }
	.dur { color: var(--text-muted); font-size: 0.8rem; }
	.chevron { color: var(--text-muted); }
	.step-body { padding: 8px; }
	.matrix { display: flex; flex-direction: column; gap: 4px; margin-bottom: 8px; }
	.matrix-item {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 0.8rem;
		padding: 4px 8px;
	}
	.empty { color: var(--text-muted); text-align: center; padding: 20px; }
</style>
