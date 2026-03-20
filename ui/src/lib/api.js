async function request(url, opts = {}) {
	const res = await fetch(url, opts);
	if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
	return res.json();
}

export const fetchBuilds = (filters = {}) => {
	const params = new URLSearchParams();
	for (const [k, v] of Object.entries(filters)) {
		if (v) params.set(k, v);
	}
	const qs = params.toString();
	return request(`/api/ci/builds${qs ? `?${qs}` : ''}`);
};

export const fetchBuild = (workflowId) => request(`/api/ci/builds/${workflowId}`);

export const fetchRepos = () => request('/api/ci/repos');

export const fetchAnalytics = (repo, days) =>
	request(`/api/ci/analytics?repo=${encodeURIComponent(repo)}&days=${days}`);

export const fetchNotifications = () => request('/api/ci/notifications');

export const triggerBuild = (owner, repo, pipeline, ref, params = {}) =>
	request(`/api/trigger/${owner}/${repo}/${pipeline}`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ ref, parameters: params })
	});

export const rerunBuild = (workflowId) =>
	request(`/api/replay/${workflowId}`, { method: 'POST' });

export const getAuthUser = () => request('/auth/me');

export const markNotificationsRead = (ids) =>
	request('/api/ci/notifications/read', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ ids })
	});
