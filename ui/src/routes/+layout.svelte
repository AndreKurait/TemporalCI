<script>
	import NavBar from '$lib/ci/NavBar.svelte';
	import '../app.css';

	let { children } = $props();
</script>

<NavBar />
<main class="content">
	{@render children()}
</main>

<style>
	.content { max-width: 1200px; margin: 0 auto; padding: 20px; }
</style>
