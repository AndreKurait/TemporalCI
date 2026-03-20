<script>
	import { fetchBuild } from '$lib/api.js';

	let { stepId = '', workflowId = '', log = '' } = $props();
	let search = $state('');
	let container;

	let lines = $derived(parseAnsi(log));
	let filtered = $derived(
		search ? lines.filter(l => l.text.toLowerCase().includes(search.toLowerCase())) : lines
	);

	$effect(() => {
		if (container && log) {
			container.scrollTop = container.scrollHeight;
		}
	});

	function parseAnsi(text) {
		if (!text) return [];
		return text.split('\n').map((raw, i) => {
			let html = raw
				.replace(/\x1b\[0m/g, '</span>')
				.replace(/\x1b\[1m/g, '<span style="font-weight:bold">')
				.replace(/\x1b\[31m/g, '<span style="color:var(--error)">')
				.replace(/\x1b\[32m/g, '<span style="color:var(--success)">')
				.replace(/\x1b\[33m/g, '<span style="color:var(--warning)">')
				.replace(/\x1b\[34m/g, '<span style="color:var(--running)">')
				.replace(/\x1b\[\d+m/g, '');
			return { text: raw, html, num: i + 1 };
		});
	}
</script>

<div class="log-viewer">
	<div class="toolbar">
		<input type="text" placeholder="Search logs..." bind:value={search} />
		<span class="count">{filtered.length} lines</span>
	</div>
	<div class="log-content" bind:this={container}>
		{#each filtered as line}
			<div class="line">
				<span class="num">{line.num}</span>
				<span class="text">{@html line.html}</span>
			</div>
		{/each}
		{#if !log}
			<div class="empty">No log output</div>
		{/if}
	</div>
</div>

<style>
	.log-viewer {
		background: #0d1117;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		overflow: hidden;
	}
	.toolbar {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px;
		border-bottom: 1px solid var(--border);
	}
	.toolbar input { flex: 1; background: #161b22; }
	.count { font-size: 0.75rem; color: var(--text-muted); }
	.log-content {
		max-height: 500px;
		overflow-y: auto;
		padding: 8px;
		font-family: var(--font-mono);
		font-size: 0.8rem;
		line-height: 1.6;
	}
	.line { display: flex; gap: 12px; }
	.line:hover { background: rgba(255,255,255,0.03); }
	.num { color: var(--text-muted); user-select: none; min-width: 3ch; text-align: right; }
	.text { white-space: pre-wrap; word-break: break-all; color: #e6edf3; }
	.empty { color: var(--text-muted); padding: 20px; text-align: center; }
</style>
