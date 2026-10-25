"use strict";
const catalog = globalThis.BridgePermissionProviders || BridgeProviders;
const selector = document.getElementById("provider");
const input = document.getElementById("origin");
const state = document.getElementById("state");
const enable = document.getElementById("enable");
const disable = document.getElementById("disable");
const scope = document.getElementById("scope");
for (const provider of catalog) {
 const option = document.createElement("option");
 option.value = provider.id;
 option.textContent = provider.label || provider.id;
 selector.appendChild(option);
}
let currentOrigin = null;
let revision = 0;
const selected = () => catalog.find(provider => provider.id === selector.value);
async function refresh() {
 const generation = ++revision;
 enable.disabled = disable.disabled = true;
 try {
  const provider = selected();
  if (!provider) throw new Error("No integration is installed");
  const sites = await BridgeSiteConfig.read();
  if (generation !== revision) return;
  currentOrigin = BridgeSiteConfig.origin(provider, sites);
  input.value = currentOrigin || "";
  input.readOnly = !provider.editableOrigin;
  scope.textContent = provider.editableOrigin ? "Choose a site for this integration." : "This integration uses a fixed site.";
  const granted = currentOrigin && await chrome.permissions.contains({origins: [BridgeSiteConfig.pattern(currentOrigin)]});
  if (generation !== revision) return;
  state.textContent = currentOrigin ? `${currentOrigin}: ${granted ? "Authorized" : "Not authorized"}` : "Enter the site you want to use.";
  enable.disabled = false;
  disable.disabled = !granted;
 } catch (error) { if (generation === revision) state.textContent = error.message; }
}
selector.onchange = () => { void refresh(); };
enable.onclick = async () => {
 // Validate and invoke request before any await, retaining the Chrome user gesture.
 const provider = selected();
 let feedback = "";
 try {
  const origin = provider.editableOrigin ? BridgeSiteConfig.normalize(input.value) : BridgeSiteConfig.normalize(provider.origin);
  const request = chrome.permissions.request({origins: [BridgeSiteConfig.pattern(origin)]});
  ++revision;
  enable.disabled = disable.disabled = true;
  if (!await request) { feedback = "Permission denied. Site was not changed."; return; }
  if (provider.editableOrigin) await BridgeSiteConfig.save(provider.id, origin);
  // Reconnect even when a previously granted origin needed no permission event.
  await chrome.runtime.sendMessage({type: "bridge:refresh-sites"});
  state.textContent = `${origin}: Authorized`;
 } catch (error) { feedback = error.message; }
 finally { await refresh(); if (feedback) state.textContent = feedback; }
};
disable.onclick = async () => {
 try {
  if (!currentOrigin) return;
  const origin = currentOrigin;
  enable.disabled = disable.disabled = true;
  await chrome.permissions.remove({origins: [BridgeSiteConfig.pattern(origin)]});
  await chrome.runtime.sendMessage({type: "bridge:refresh-sites"});
 } catch (error) { state.textContent = error.message; }
 finally { await refresh(); }
};
void refresh();
