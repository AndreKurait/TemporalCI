<script>
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { fetchRepos, triggerBuild } from '$lib/api.js';

	let repos = $state([]);
	let selectedRepo = $state('');
	let pipeline = $state('default');
	let ref = $state('main');
	let params = $state([{ key: '', value: '' }]);
	let submitting = $state(false);
	let error = $state('');

	onMount(async () => {
		try {
			const data = await fetchRepos();
			repos = Array.isArray(data) ? data : data.repos || [];
			const q = $page.url.searchParams.get('repo');
			if (q) selectedRepo = q;
		} catch {}
	});

	function addParam() {
		params = [...params, { key: '', value: '' }];
	}

	function removeParam(i) {
		params = params.filter((_, idx) => idx !== i);
	}

	async function submit() {
		if (!selectedRepo) { error = 'Select a repository'; return; }
		error = '';
		submitting = true;
		try {
			const [owner, repo] = selectedRepo.split('/');
			const p = {};
			for (const { key, value } of params) {
				if (key) p[key] = value;
			}
			const result = await triggerBuild(owner, repo, pipeline, ref, p);
			if (result?.workflowId) {
				goto(`/ci/builds/${result.workflowId}`);
			}
		} catch (e) {
			error = e.message;
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head><title>Trigger Build — TemporalCI</title></svelte:head>

<div class="page">
	<h2>Trigger Build</h2>

	<div class="card form">
		<label>
			Repository
			<select bind:value={selectedRepo}>
				<option value="">Select repo...</option>
				{#each repos as r}
					<option value={r.fullName || r.name}>{r.fullName || r.name}</option>
				{/each}
			</select>
		</label>

		<label>
			Pipeline
			<input type="text" bind:value={pipeline} placeholder="default" />
		</label>

		<label>
			Ref (branch/tag)
			<input type="text" bind:value={ref} placeholder="main" />
		</label>

		<fieldset>
			<legend>Parameters</legend>
			{#each params as param, i}
				<div class="param-row">
					<input type="text" placeholder="Key" bind:value={param.key} />
					<input type="text" placeholder="Value" bind:value={param.value} />
					<button onclick={() => removeParam(i)}>✕</button>
				</div>
			{/each}
			<button onclick={addParam}>+ Add parameter</button>
		</fieldset>

		{#if error}<p class="error">{error}</p>{/if}

		<button class="submit" onclick={submit} disabled={submitting}>
			{submitting ? 'Triggering...' : 'Trigger Build'}
		</button>
	</div>
</div>

<style>
	.page h2 { margin-bottom: 16px; font-size: 1.25rem; }
	.form { display: flex; flex-direction: column; gap: 14px; max-width: 500px; }
	label { display: flex; flex-direction: column; gap: 4px; font-size: 0.875rem; }
	fieldset { border: 1px solid var(--border); border-radius: var(--radius); padding: 12px; }
	legend { font-size: 0.875rem; padding: 0 4px; }
	.param-row { display: flex; gap: 6px; margin-bottom: 6px; }
	.param-row input { flex: 1; }
	.param-row button { padding: 4px 8px; font-size: 0.8rem; }
	.error { color: var(--error); font-size: 0.8rem; }
	.submit {
		background: var(--text-link);
		color: white;
		border-color: var(--text-link);
		padding: 8px 16px;
		font-weight: 600;
	}
	.submit:disabled { opacity: 0.6; cursor: not-allowed; }
</style>
