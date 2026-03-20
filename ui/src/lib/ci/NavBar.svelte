<script>
	import { fetchNotifications, getAuthUser } from '$lib/api.js';
	import { onMount } from 'svelte';

	let notifCount = $state(0);
	let user = $state(null);

	onMount(async () => {
		try {
			const data = await fetchNotifications();
			notifCount = data?.unread ?? data?.length ?? 0;
		} catch {}
		try {
			user = await getAuthUser();
		} catch {}
	});
</script>

<nav class="navbar">
	<a href="/ci/builds" class="logo">⚡ TemporalCI</a>
	<div class="links">
		<a href="/ci/builds">CI</a>
		<a href="/ci/repos">Repos</a>
		<a href="/ci/triggers">Triggers</a>
		<a href="/ci/analytics">Analytics</a>
		<a href="/namespaces/default/workflows" class="external">Temporal UI ↗</a>
	</div>
	<div class="right">
		<a href="/ci/builds" class="bell" title="Notifications">
			🔔{#if notifCount > 0}<span class="badge">{notifCount}</span>{/if}
		</a>
		{#if user}
			<span class="user">{user.login || user.name}</span>
		{:else}
			<a href="/auth/login">Login</a>
		{/if}
	</div>
</nav>

<style>
	.navbar {
		display: flex;
		align-items: center;
		gap: 16px;
		padding: 0 20px;
		height: 48px;
		background: var(--bg-surface);
		border-bottom: 1px solid var(--border);
		font-size: 0.875rem;
	}
	.logo {
		font-weight: 700;
		font-size: 1rem;
		color: var(--text);
		margin-right: 8px;
	}
	.logo:hover { text-decoration: none; }
	.links { display: flex; gap: 16px; }
	.links a { color: var(--text-muted); }
	.links a:hover { color: var(--text); text-decoration: none; }
	.external { font-size: 0.8rem; }
	.right { margin-left: auto; display: flex; align-items: center; gap: 12px; }
	.bell { position: relative; text-decoration: none; }
	.badge {
		position: absolute;
		top: -6px;
		right: -8px;
		background: var(--error);
		color: white;
		font-size: 0.65rem;
		padding: 1px 4px;
		border-radius: 8px;
	}
	.user { color: var(--text-muted); }
</style>
