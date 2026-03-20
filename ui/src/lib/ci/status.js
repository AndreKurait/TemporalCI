const icons = {
	passed: '✅', success: '✅', completed: '✅',
	failed: '❌', error: '❌',
	running: '🔄', in_progress: '🔄',
	paused: '⏸️',
	skipped: '⏭️', cancelled: '⏭️',
	queued: '⏳', pending: '⏳',
	timed_out: '⏰', unknown: '⬜'
};

const colors = {
	passed: 'var(--success)', success: 'var(--success)', completed: 'var(--success)',
	failed: 'var(--error)', error: 'var(--error)',
	running: 'var(--running)', in_progress: 'var(--running)',
	paused: 'var(--warning)',
	skipped: 'var(--neutral)', cancelled: 'var(--neutral)',
	queued: 'var(--neutral)', pending: 'var(--neutral)',
	timed_out: 'var(--error)', unknown: 'var(--neutral)'
};

export const statusIcon = (status) => icons[status] || '⬜';
export const statusColor = (status) => colors[status] || 'var(--neutral)';

export function formatDuration(seconds) {
	if (!seconds || seconds < 0) return '—';
	const s = Math.floor(seconds);
	if (s < 60) return `${s}s`;
	const m = Math.floor(s / 60);
	const rs = s % 60;
	if (m < 60) return `${m}m ${rs}s`;
	const h = Math.floor(m / 60);
	return `${h}h ${m % 60}m`;
}

export function formatRelativeTime(date) {
	const diff = Date.now() - new Date(date).getTime();
	const s = Math.floor(diff / 1000);
	if (s < 60) return `${s}s ago`;
	const m = Math.floor(s / 60);
	if (m < 60) return `${m}m ago`;
	const h = Math.floor(m / 60);
	if (h < 24) return `${h}h ago`;
	const d = Math.floor(h / 24);
	return `${d}d ago`;
}
