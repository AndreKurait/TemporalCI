<script>
	import { statusColor } from './status.js';

	let { steps = [], onNodeClick = () => {} } = $props();

	let hasDeps = $derived(steps.some(s => s.dependsOn?.length));

	// Layout: assign columns based on dependency depth
	let layout = $derived(() => {
		if (!hasDeps) return { nodes: [], edges: [], width: 0, height: 0 };

		const byName = new Map(steps.map(s => [s.name, s]));
		const depths = new Map();

		function getDepth(name) {
			if (depths.has(name)) return depths.get(name);
			const step = byName.get(name);
			if (!step?.dependsOn?.length) { depths.set(name, 0); return 0; }
			const d = 1 + Math.max(...step.dependsOn.map(getDepth));
			depths.set(name, d);
			return d;
		}

		steps.forEach(s => getDepth(s.name));

		const cols = new Map();
		for (const [name, d] of depths) {
			if (!cols.has(d)) cols.set(d, []);
			cols.get(d).push(name);
		}

		const nodeW = 140, nodeH = 40, gapX = 60, gapY = 20, padX = 20, padY = 20;
		const nodes = [];
		const positions = new Map();

		for (const [col, names] of cols) {
			names.forEach((name, row) => {
				const x = padX + col * (nodeW + gapX);
				const y = padY + row * (nodeH + gapY);
				positions.set(name, { x, y });
				const step = byName.get(name);
				nodes.push({ name, x, y, w: nodeW, h: nodeH, step, matrixCount: step?.matrix?.length });
			});
		}

		const edges = [];
		for (const step of steps) {
			if (!step.dependsOn) continue;
			for (const dep of step.dependsOn) {
				const from = positions.get(dep);
				const to = positions.get(step.name);
				if (from && to) {
					edges.push({ x1: from.x + nodeW, y1: from.y + nodeH / 2, x2: to.x, y2: to.y + nodeH / 2 });
				}
			}
		}

		const maxCol = Math.max(...[...cols.keys()]);
		const maxRows = Math.max(...[...cols.values()].map(v => v.length));
		const width = padX * 2 + (maxCol + 1) * (nodeW + gapX) - gapX;
		const height = padY * 2 + maxRows * (nodeH + gapY) - gapY;

		return { nodes, edges, width, height };
	});
</script>

{#if hasDeps}
	{@const l = layout()}
	<div class="dag-container">
		<svg width={l.width} height={l.height}>
			{#each l.edges as e}
				<path
					d="M{e.x1},{e.y1} C{e.x1 + 30},{e.y1} {e.x2 - 30},{e.y2} {e.x2},{e.y2}"
					fill="none"
					stroke="var(--border)"
					stroke-width="2"
				/>
			{/each}
			{#each l.nodes as node}
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<g role="button" tabindex="0" onclick={() => onNodeClick(node.name)} style="cursor:pointer">
					<rect
						x={node.x} y={node.y}
						width={node.w} height={node.h}
						rx="4"
						fill="var(--bg-card)"
						stroke={statusColor(node.step?.status)}
						stroke-width="2"
					/>
					<text
						x={node.x + node.w / 2} y={node.y + node.h / 2 + 4}
						text-anchor="middle"
						fill="var(--text)"
						font-size="11"
					>{node.name.length > 16 ? node.name.slice(0, 15) + '…' : node.name}</text>
					{#if node.matrixCount}
						<circle cx={node.x + node.w - 8} cy={node.y + 8} r="8" fill="var(--running)" />
						<text x={node.x + node.w - 8} y={node.y + 12} text-anchor="middle" fill="white" font-size="9">{node.matrixCount}</text>
					{/if}
				</g>
			{/each}
		</svg>
	</div>
{/if}

<style>
	.dag-container {
		overflow-x: auto;
		padding: 16px;
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
	}
</style>
