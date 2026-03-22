const state = {
  items: [],
  page: 1,
  pageSize: 40,
  total: 0,
  selectedID: "",
  sourceJSON: "",
};

const els = {
  metaCount: document.getElementById("meta-count"),
  metaRange: document.getElementById("meta-range"),
  metaPath: document.getElementById("meta-path"),
  search: document.getElementById("search"),
  startDate: document.getElementById("start-date"),
  endDate: document.getElementById("end-date"),
  hasPhotos: document.getElementById("has-photos"),
  apply: document.getElementById("apply"),
  random: document.getElementById("random"),
  refresh: document.getElementById("refresh"),
  prev: document.getElementById("prev"),
  next: document.getElementById("next"),
  pageLabel: document.getElementById("page-label"),
  resultsLabel: document.getElementById("results-label"),
  resultsSubtitle: document.getElementById("results-subtitle"),
  items: document.getElementById("items"),
  detailEmpty: document.getElementById("detail-empty"),
  detailView: document.getElementById("detail-view"),
  detailHero: document.getElementById("detail-hero"),
  detailDate: document.getElementById("detail-date"),
  detailCategory: document.getElementById("detail-category"),
  detailVenue: document.getElementById("detail-venue"),
  detailLocation: document.getElementById("detail-location"),
  detailShout: document.getElementById("detail-shout"),
  detailCompanions: document.getElementById("detail-companions"),
  detailPhotos: document.getElementById("detail-photos"),
  photoCount: document.getElementById("photo-count"),
  detailFacts: document.getElementById("detail-facts"),
  viewSource: document.getElementById("view-source"),
  sourceModal: document.getElementById("source-modal"),
  closeSource: document.getElementById("close-source"),
  detailJSON: document.getElementById("detail-json"),
};

async function request(path) {
  const response = await fetch(path);
  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(payload.error || response.statusText);
  }
  return response.json();
}

function buildParams() {
  const params = new URLSearchParams({
    page: String(state.page),
    page_size: String(state.pageSize),
  });
  const q = els.search.value.trim();
  if (q) params.set("q", q);
  if (els.startDate.value) params.set("start_date", els.startDate.value);
  if (els.endDate.value) params.set("end_date", els.endDate.value);
  if (els.hasPhotos.checked) params.set("has_photos", "1");
  return params;
}

function buildFilterSummary() {
  const active = [];
  if (els.search.value.trim()) active.push(`search: ${els.search.value.trim()}`);
  if (els.startDate.value) active.push(`from ${els.startDate.value}`);
  if (els.endDate.value) active.push(`to ${els.endDate.value}`);
  if (els.hasPhotos.checked) active.push("with pictures");
  return active.length > 0 ? active.join(" • ") : "Full archive view";
}

async function loadMeta() {
  const meta = await request("/api/meta");
  els.metaCount.textContent = `${Number(meta.count || 0).toLocaleString()} archived check-ins`;
  els.metaRange.textContent = meta.min_date && meta.max_date
    ? `${meta.min_date.slice(0, 10)} to ${meta.max_date.slice(0, 10)}`
    : "No date range available";
  els.metaPath.textContent = meta.db_path || "";
  if (meta.min_date) els.startDate.min = meta.min_date.slice(0, 10);
  if (meta.max_date) els.startDate.max = meta.max_date.slice(0, 10);
  if (meta.min_date) els.endDate.min = meta.min_date.slice(0, 10);
  if (meta.max_date) els.endDate.max = meta.max_date.slice(0, 10);
}

async function loadList() {
  const payload = await request(`/api/checkins?${buildParams().toString()}`);
  state.items = Array.isArray(payload.items) ? payload.items : [];
  state.total = Number(payload.total || 0);
  renderList();

  if (state.items.length === 0) {
    showEmpty("No matching check-ins.");
    return;
  }

  const currentStillVisible = state.selectedID && state.items.some((item) => item.id === state.selectedID);
  await selectItem(currentStillVisible ? state.selectedID : state.items[0].id);
}

function renderList() {
  els.items.innerHTML = "";
  els.resultsLabel.textContent = `${state.total.toLocaleString()} matching check-ins`;
  els.resultsSubtitle.textContent = buildFilterSummary();

  const totalPages = Math.max(1, Math.ceil(state.total / state.pageSize));
  els.pageLabel.textContent = `Page ${state.page} of ${totalPages}`;
  els.prev.disabled = state.page <= 1;
  els.next.disabled = state.page >= totalPages;

  if (state.items.length === 0) {
    const empty = document.createElement("div");
    empty.className = "empty";
    empty.textContent = "No check-ins found for the current filters.";
    els.items.appendChild(empty);
    return;
  }

  for (const item of state.items) {
    const button = document.createElement("button");
    button.className = `item${item.id === state.selectedID ? " active" : ""}`;
    button.type = "button";
    button.addEventListener("click", async () => {
      await selectItem(item.id);
    });

    const thumb = item.photos && item.photos[0]
      ? `<img src="${escapeAttr(item.photos[0].thumb_url || item.photos[0].url)}" alt="">`
      : `<span>No photo</span>`;

    const location = [item.city, item.state, item.country].filter(Boolean).join(", ");
    const pills = [];
    if (item.category) pills.push(item.category);
    if (item.source) pills.push(item.source);
    if (item.photos && item.photos.length > 0) pills.push(`${item.photos.length} photo${item.photos.length === 1 ? "" : "s"}`);

    button.innerHTML = `
      <div class="item-row">
        <div class="item-thumb">${thumb}</div>
        <div>
          <div class="item-top">
            <div class="item-title">${escapeHTML(item.venue_name || "(No venue name)")}</div>
            <div class="item-date">${formatDate(item.date)}</div>
          </div>
          <div class="item-meta">
            ${escapeHTML(location || "Unknown location")}
            ${item.shout ? `<br>${escapeHTML(item.shout)}` : ""}
          </div>
          <div class="pill-row">
            ${pills.map((pill) => `<span class="pill">${escapeHTML(pill)}</span>`).join("")}
          </div>
        </div>
      </div>
    `;
    els.items.appendChild(button);
  }
}

async function selectItem(id) {
  if (!id) return;
  state.selectedID = id;
  renderList();
  const payload = await request(`/api/checkins/${encodeURIComponent(id)}`);
  renderDetail(payload);
}

function renderDetail(payload) {
  const summary = payload && payload.summary ? payload.summary : {};
  const raw = payload && payload.raw ? payload.raw : {};
  const photos = Array.isArray(summary.photos) ? summary.photos : [];

  state.sourceJSON = payload.pretty || JSON.stringify(raw, null, 2);
  els.detailJSON.textContent = state.sourceJSON;

  els.detailEmpty.classList.add("hidden");
  els.detailView.classList.remove("hidden");

  els.detailDate.textContent = formatDate(summary.date);
  els.detailCategory.textContent = summary.category || "Check-in";
  els.detailVenue.textContent = summary.venue_name || "(No venue name)";
  els.detailLocation.textContent = [summary.city, summary.state, summary.country].filter(Boolean).join(", ") || "Unknown location";

  if (summary.shout) {
    els.detailShout.textContent = summary.shout;
    els.detailShout.classList.remove("hidden");
  } else {
    els.detailShout.textContent = "";
    els.detailShout.classList.add("hidden");
  }

  const people = Array.isArray(summary.people) ? summary.people : [];
  if (people.length > 0) {
    els.detailCompanions.innerHTML = people.map((name) => `<span class="companion">${escapeHTML(name)}</span>`).join("");
    els.detailCompanions.classList.remove("hidden");
  } else {
    els.detailCompanions.innerHTML = "";
    els.detailCompanions.classList.add("hidden");
  }

  renderHero(photos);
  renderPhotoGrid(photos);
  renderFacts(summary, raw);
}

function renderHero(photos) {
  if (!photos.length) {
    els.detailHero.className = "detail-hero empty";
    els.detailHero.innerHTML = "<div>No photos attached to this check-in.</div>";
    return;
  }

  els.detailHero.className = "detail-hero";
  const shown = photos.slice(0, 3);
  els.detailHero.innerHTML = shown.map((photo) => `
    <a class="hero-photo" href="${escapeAttr(photo.url)}" target="_blank" rel="noreferrer">
      <img src="${escapeAttr(photo.thumb_url || photo.url)}" alt="">
    </a>
  `).join("");
}

function renderPhotoGrid(photos) {
  els.photoCount.textContent = photos.length > 0 ? `${photos.length} attached` : "";
  if (!photos.length) {
    els.detailPhotos.innerHTML = `<div class="empty">No pictures on this check-in.</div>`;
    return;
  }
  els.detailPhotos.innerHTML = photos.map((photo) => `
    <a class="photo-card" href="${escapeAttr(photo.url)}" target="_blank" rel="noreferrer">
      <img src="${escapeAttr(photo.thumb_url || photo.url)}" alt="">
      <div class="photo-meta">${escapeHTML(photo.width && photo.height ? `${photo.width}×${photo.height}` : "Open full image")}</div>
    </a>
  `).join("");
}

function renderFacts(summary, raw) {
  const venue = raw.venue || {};
  const location = venue.location || {};
  const facts = [
    ["ID", summary.id || ""],
    ["Source", summary.source || "Unknown"],
    ["Address", location.address || ""],
    ["Lat/Lng", typeof location.lat === "number" && typeof location.lng === "number" ? `${location.lat}, ${location.lng}` : ""],
    ["Timezone Offset", raw.timeZoneOffset != null ? String(raw.timeZoneOffset) : ""],
    ["Visibility", raw.visibility || ""],
  ].filter(([, value]) => value);

  els.detailFacts.innerHTML = facts.map(([label, value]) => `
    <div>
      <dt>${escapeHTML(label)}</dt>
      <dd>${escapeHTML(value)}</dd>
    </div>
  `).join("");
}

function showEmpty(message) {
  els.detailEmpty.textContent = message;
  els.detailEmpty.classList.remove("hidden");
  els.detailView.classList.add("hidden");
}

async function loadRandom() {
  const params = buildParams();
  params.delete("page");
  params.delete("page_size");
  const payload = await request(`/api/random?${params.toString()}`);
  if (payload && payload.summary && payload.summary.id) {
    state.selectedID = payload.summary.id;
    renderList();
    renderDetail(payload);
  }
}

function formatDate(value) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function escapeAttr(value) {
  return escapeHTML(value);
}

async function applyFilters() {
  state.page = 1;
  await loadList();
}

els.apply.addEventListener("click", applyFilters);
els.refresh.addEventListener("click", async () => {
  await loadMeta();
  await loadList();
});
els.random.addEventListener("click", loadRandom);
els.prev.addEventListener("click", async () => {
  if (state.page > 1) {
    state.page -= 1;
    await loadList();
  }
});
els.next.addEventListener("click", async () => {
  const totalPages = Math.max(1, Math.ceil(state.total / state.pageSize));
  if (state.page < totalPages) {
    state.page += 1;
    await loadList();
  }
});
els.search.addEventListener("keydown", async (event) => {
  if (event.key === "Enter") await applyFilters();
});
els.startDate.addEventListener("change", applyFilters);
els.endDate.addEventListener("change", applyFilters);
els.hasPhotos.addEventListener("change", applyFilters);
els.viewSource.addEventListener("click", () => {
  if (typeof els.sourceModal.showModal === "function") {
    els.sourceModal.showModal();
  } else {
    els.sourceModal.setAttribute("open", "open");
  }
});
els.closeSource.addEventListener("click", () => {
  if (typeof els.sourceModal.close === "function") {
    els.sourceModal.close();
  } else {
    els.sourceModal.removeAttribute("open");
  }
});
els.sourceModal.addEventListener("click", (event) => {
  if (event.target === els.sourceModal && typeof els.sourceModal.close === "function") {
    els.sourceModal.close();
  }
});

async function boot() {
  try {
    await loadMeta();
    await loadList();
  } catch (error) {
    console.error(error);
    showEmpty(error.message || "Failed to load archive.");
  }
}

boot();
