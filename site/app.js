const order = ["alpha", "beta", "stable"];
const labels = { alpha: "Alpha", beta: "Beta", stable: "Stable" };

function escapeHTML(value) {
  return String(value).replace(/[&<>'"]/g, ch => ({"&":"&amp;","<":"&lt;",">":"&gt;","'":"&#39;",'"':"&quot;"}[ch]));
}

async function load() {
  const container = document.querySelector("#channels");
  try {
    const response = await fetch("status.json", { cache: "no-store" });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const status = await response.json();
    document.querySelector("#serviceState").classList.add("ok");
    document.querySelector("#serviceState").lastChild.textContent = "Online";
    document.querySelector("#updatedAt").textContent = status.updatedAt ? `Updated ${new Date(status.updatedAt).toLocaleString()}` : "No releases";
    container.innerHTML = order.map(channel => {
      const item = status.channels?.[channel];
      if (!item) {
        return `<article class="channel"><div class="channel-top"><span class="badge">${labels[channel]}</span></div><p class="version empty">Not published</p><p class="meta">No update has been assigned to this channel.</p></article>`;
      }
      const published = new Date(item.publishedAt).toLocaleString();
      return `<article class="channel">
        <div class="channel-top"><span class="badge">${labels[channel]}</span>${item.mandatory ? '<span class="badge">Required</span>' : ''}</div>
        <p class="version">${escapeHTML(item.version)}</p>
        <p class="meta">Published ${escapeHTML(published)}<br>Rollout ${Number(item.rolloutPercent)}%</p>
        <a href="${escapeHTML(item.manifest)}">View signed manifest →</a>
      </article>`;
    }).join("");
  } catch (error) {
    document.querySelector("#serviceState").lastChild.textContent = "Not initialized";
    container.innerHTML = order.map(channel => `<article class="channel"><div class="channel-top"><span class="badge">${labels[channel]}</span></div><p class="version empty">Not published</p><p class="meta">The release service is ready but this channel has no manifest yet.</p></article>`).join("");
  }
}
load();
