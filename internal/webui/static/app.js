async function loadKeys() {
	const res = await fetch("/api/keys");
	const items = await res.json();

	const tbody = document.getElementById("key-list-body");
	tbody.replaceChildren();

	for (const key of items) {
		const row = document.createElement("tr");
		for (const value of [
			key.key_id,
			key.owner,
			formatDateTime(key.created_at),
			formatExpiryDate(key.expires_at),
		]) {
			const cell = document.createElement("td");
			cell.textContent = value;
			row.appendChild(cell);
		}

		const actionCell = document.createElement("td");

		const usageButton = document.createElement("button");
		usageButton.type = "button";
		usageButton.textContent = "Usage";
		usageButton.addEventListener("click", () => showUsage(key.key_id));
		actionCell.appendChild(usageButton);

		const revokeButton = document.createElement("button");
		revokeButton.type = "button";
		revokeButton.className = "revoke";
		revokeButton.textContent = "Revoke";
		revokeButton.addEventListener("click", () => revokeKey(key.key_id, revokeButton));
		actionCell.appendChild(revokeButton);

		row.appendChild(actionCell);

		tbody.appendChild(row);
	}
}

async function revokeKey(keyID, button) {
	const errorEl = document.getElementById("revoke-key-error");
	errorEl.hidden = true;

	if (!confirm(`Revoke key ${keyID}? This cannot be undone.`)) {
		return;
	}

	// Disabled for the duration of the request so a second click (e.g. a
	// double-click, or one queued while the first request is in flight)
	// can't fire a duplicate DELETE against a key the first request already
	// revoked.
	button.disabled = true;

	let res;
	try {
		res = await fetch(`/api/keys/${encodeURIComponent(keyID)}`, { method: "DELETE" });
	} catch {
		errorEl.textContent = "Failed to reach the server.";
		errorEl.hidden = false;
		button.disabled = false;
		return;
	}

	if (!res.ok) {
		const data = await res.json().catch(() => ({}));
		errorEl.textContent = data.error || "Failed to revoke key.";
		errorEl.hidden = false;
		button.disabled = false;
		return;
	}

	loadKeys();
}

function formatDateTime(value) {
	return value ? new Date(value).toLocaleString() : "—";
}

// Expiry is chosen as a calendar date (the <input type="date"> has no
// time-of-day component) and sent as UTC midnight. Displaying it in the
// viewer's local time zone can shift it to the previous day west of UTC, so
// it's rendered back out as a UTC date instead of a localized instant.
function formatExpiryDate(value) {
	return value ? new Date(value).toLocaleDateString(undefined, { timeZone: "UTC" }) : "—";
}

document.getElementById("create-key-form").addEventListener("submit", async (event) => {
	event.preventDefault();

	const errorEl = document.getElementById("create-key-error");
	const bannerEl = document.getElementById("new-key-banner");
	errorEl.hidden = true;
	bannerEl.hidden = true;

	const form = event.target;
	const body = { owner: form.owner.value };
	if (form.expires_at.value) {
		body.expires_at = new Date(form.expires_at.value).toISOString();
	}

	let res;
	try {
		res = await fetch("/api/keys", {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify(body),
		});
	} catch {
		errorEl.textContent = "Failed to reach the server.";
		errorEl.hidden = false;
		return;
	}

	const data = await res.json().catch(() => ({}));

	if (!res.ok) {
		errorEl.textContent = data.error || "Failed to create key.";
		errorEl.hidden = false;
		return;
	}

	document.getElementById("new-key-value").textContent = data.key;
	bannerEl.hidden = false;
	form.reset();
	loadKeys();
});

document.getElementById("test-key-form").addEventListener("submit", async (event) => {
	event.preventDefault();

	const resultEl = document.getElementById("test-key-result");
	resultEl.hidden = true;
	resultEl.className = "key-test-result";

	const form = event.target;
	let res;
	try {
		res = await fetch("/api/keys/validate", {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({ key: form.key.value }),
		});
	} catch {
		resultEl.textContent = "Failed to reach the server.";
		resultEl.classList.add("invalid");
		resultEl.hidden = false;
		return;
	}

	const data = await res.json().catch(() => ({}));

	if (!res.ok) {
		resultEl.textContent = data.error || "Failed to test key.";
		resultEl.classList.add("invalid");
		resultEl.hidden = false;
		return;
	}

	if (data.valid) {
		resultEl.textContent = `Valid — owned by "${data.owner}".`;
		resultEl.classList.add("valid");
	} else {
		resultEl.textContent = `Invalid — ${data.reason}.`;
		resultEl.classList.add("invalid");
	}
	resultEl.hidden = false;
});

// showUsage fetches a key's usage graph, sourced live from Prometheus by the
// backend (ADR-0004) rather than anything stored in Postgres, and renders it
// as a simple line chart. A non-2xx response (the backend's degraded state
// for an unreachable Prometheus) is shown as an error rather than a graph.
async function showUsage(keyID) {
	const section = document.getElementById("key-usage");
	const loadingEl = document.getElementById("usage-loading");
	const errorEl = document.getElementById("usage-error");
	const emptyEl = document.getElementById("usage-empty");
	const canvas = document.getElementById("usage-canvas");

	document.getElementById("usage-key-id").textContent = keyID;
	section.hidden = false;
	loadingEl.hidden = false;
	errorEl.hidden = true;
	emptyEl.hidden = true;
	canvas.getContext("2d").clearRect(0, 0, canvas.width, canvas.height);

	let res;
	try {
		res = await fetch(`/api/keys/${encodeURIComponent(keyID)}/usage`);
	} catch {
		loadingEl.hidden = true;
		errorEl.textContent = "Failed to reach the server.";
		errorEl.hidden = false;
		return;
	}

	loadingEl.hidden = true;

	if (!res.ok) {
		const data = await res.json().catch(() => ({}));
		errorEl.textContent = data.error || "Usage data is currently unavailable.";
		errorEl.hidden = false;
		return;
	}

	const points = await res.json();
	if (points.length === 0) {
		emptyEl.hidden = false;
		return;
	}

	drawUsageChart(canvas, points);
}

function drawUsageChart(canvas, points) {
	const ctx = canvas.getContext("2d");
	const { width, height } = canvas;
	const padding = 24;

	ctx.clearRect(0, 0, width, height);

	ctx.strokeStyle = "#888";
	ctx.lineWidth = 1;
	ctx.beginPath();
	ctx.moveTo(padding, padding);
	ctx.lineTo(padding, height - padding);
	ctx.lineTo(width - padding, height - padding);
	ctx.stroke();

	// maxValue is floored at a small positive number so a fully idle key
	// (every value 0) still draws a flat line at the axis instead of dividing
	// by zero.
	const maxValue = Math.max(...points.map((p) => p.value), 0.0001);
	const plotWidth = width - padding * 2;
	const plotHeight = height - padding * 2;

	ctx.strokeStyle = "#2a6ebb";
	ctx.lineWidth = 2;
	ctx.beginPath();
	points.forEach((point, i) => {
		const x = padding + (plotWidth * i) / Math.max(points.length - 1, 1);
		const y = height - padding - (plotHeight * point.value) / maxValue;
		if (i === 0) {
			ctx.moveTo(x, y);
		} else {
			ctx.lineTo(x, y);
		}
	});
	ctx.stroke();
}

document.getElementById("usage-close").addEventListener("click", () => {
	document.getElementById("key-usage").hidden = true;
});

loadKeys();
