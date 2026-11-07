'use strict';

// No token persistence, cookies, third-party scripts, or server-sent HTML.
// All untrusted inventory fields are rendered through textContent.
const $ = (selector) => document.querySelector(selector);
const tokenInput = $('#auth-token');
let token = '';
let identity = null;
let view = 'overview';
let inventory = { users: [], devices: [], gateways: [], policies: [], tokens: [] };

const pages = {
  overview: ['Overview', 'A clear view of private-network inventory and desired state.'],
  users: ['Users', 'Manage user status, expiration and device allowances.'],
  devices: ['Devices', 'Register unique WireGuard public keys and revoke inventory credentials.'],
  gateways: ['Gateways', 'Maintain a trusted list of direct-IP WireGuard endpoints.'],
  policies: ['Policies', 'Configure desired transport and DNS behavior for each user.'],
  audit: ['Audit trail', 'Review administrative changes; no VPN browsing history is collected.'],
  access: ['Access tokens', 'Issue and revoke scoped control-plane API tokens.']
};
const permittedToWrite = () => identity && ['owner','admin'].includes(identity.role);
const isOwner = () => identity && identity.role === 'owner';

function el(tag, className, content) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (content !== undefined && content !== null) node.textContent = String(content);
  return node;
}
function append(parent, ...children) { children.forEach((c) => parent.append(c)); return parent; }
function badge(status) {
  const ok = ['active','enabled','owner'];
  const caution = ['suspended','disabled','operator'];
  return el('span', 'tag ' + (ok.includes(status) ? 'good' : caution.includes(status) ? 'warn' : 'bad'), status);
}
function action(label, callback, dangerous=false) {
  const button = el('button', 'action' + (dangerous ? ' danger' : ''), label);
  button.type = 'button';
  button.addEventListener('click', callback);
  return button;
}
function formatDate(value) {
  if (!value) return '—';
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? '—' : d.toLocaleString();
}
function mono(value) {
  const e = el('span','mono truncate',value || '—');
  e.title = String(value || '');
  return e;
}
function toast(message, isError=false) {
  const box = $('#toast');
  box.textContent = message;
  box.className = 'toast' + (isError ? ' error' : '');
  // Explicitly no background task / polling: disappears on next action.
}
async function request(path, options={}) {
  const headers = { 'Authorization': 'Bearer ' + token };
  if (options.body !== undefined) headers['Content-Type'] = 'application/json';
  let response;
  try {
    response = await fetch(path, {
      method: options.method || 'GET', headers,
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
      cache: 'no-store'
    });
  } catch (_) { throw new Error('Cannot reach the control-plane API.'); }
  let data = {};
  try { data = await response.json(); } catch (_) {}
  if (!response.ok) {
    if (response.status === 401) signOut(false);
    throw new Error(data.error?.message || 'Request failed (' + response.status + ').');
  }
  return data;
}
function signOut(message=true) {
  token = '';
  identity = null;
  tokenInput.value = '';
  $('#shell').classList.add('hidden');
  $('#login').classList.remove('hidden');
  $('#toast').classList.add('hidden');
  if (message) $('#login-error').textContent = 'Signed out. Re-enter the token to continue.';
}
$('#logout').addEventListener('click', () => signOut());
$('#login-form').addEventListener('submit', async (event) => {
  event.preventDefault();
  $('#login-error').textContent = '';
  token = tokenInput.value.trim();
  try {
    identity = await request('/api/v1/me');
    tokenInput.value = '';
    $('#login').classList.add('hidden');
    $('#shell').classList.remove('hidden');
    $('#actor-role').textContent = identity.role + ' / ' + identity.name;
    document.querySelectorAll('.owner-only').forEach((item) => item.classList.toggle('hidden', !isOwner()));
    await load('overview');
  } catch (err) {
    $('#login-error').textContent = err.message;
    token = '';
  }
});
document.querySelectorAll('.nav-item').forEach((button) => button.addEventListener('click', async () => {
  if (button.dataset.view === 'access' && !isOwner()) return;
  try { await load(button.dataset.view); } catch (err) { toast(err.message, true); }
}));

function card(title, content, rightText='') {
  const c = el('section','card');
  const heading = el('div','card-head');
  append(heading, el('h2','',title), el('span','subtle',rightText));
  append(c,heading,content);
  return c;
}
function table(headers, records, columns, emptyText) {
  const wrap = el('div','table-wrap');
  if (!records.length) {
    const empty = el('div','empty');
    append(empty, el('strong','','Nothing here yet'), el('span','',emptyText || 'Use Create to add your first item.'));
    return append(wrap,empty);
  }
  const t = el('table','data-table');
  const thead = el('thead');
  const tr = el('tr');
  headers.forEach((h) => append(tr,el('th','',h)));
  append(thead,tr);
  const tbody=el('tbody');
  for (const item of records) {
    const row=el('tr');
    columns.forEach((fn) => {
      const td=el('td');
      const value=fn(item);
      if (value instanceof Node) td.append(value);
      else td.textContent=String(value ?? '—');
      row.append(td);
    });
    tbody.append(row);
  }
  return append(wrap, append(t,thead,tbody));
}
function actions(...buttons) { return append(el('div','actions'),...buttons); }

async function load(nextView=view) {
  view=nextView;
  const [title,description]=pages[view];
  $('#page-title').textContent=title;
  $('#page-subtitle').textContent=description;
  $('#crumb').textContent=title;
  document.querySelectorAll('.nav-item').forEach((b) => b.classList.toggle('active',b.dataset.view===view));
  const newButton=$('#new-item');
  const creators={users:newUser,devices:newDevice,gateways:newGateway,policies:newPolicy,access:newToken};
  const canCreate=typeof creators[view]==='function' && (view==='access'?isOwner():permittedToWrite());
  newButton.classList.toggle('hidden',!canCreate);
  newButton.onclick=canCreate?creators[view]:null;
  const content=$('#page-content');
  content.replaceChildren(el('p','muted','Loading…'));
  try {
    const renderers={
      overview:renderOverview,users:renderUsers,devices:renderDevices,
      gateways:renderGateways,policies:renderPolicies,audit:renderAudit,access:renderTokens
    };
    content.replaceChildren(await renderers[view]());
  } catch (err) {
    content.replaceChildren(el('div','warning',err.message));
    throw err;
  }
}
async function renderOverview() {
  const data=await request('/api/v1/overview');
  const fragment=el('div');
  const metrics=el('div','metrics');
  [
    ['Users',data.users_total,data.users_active+' active'],
    ['Devices',data.devices_total,data.devices_active+' active'],
    ['Gateways',data.gateways_total,data.gateways_enabled+' enabled'],
    ['Data-plane link','OFFLINE','Not implemented in alpha']
  ].forEach(([label,value,hint]) => {
    const box=el('div','metric');
    append(box,el('div','label',label),el('div','value',value),el('div','hint',hint));
    metrics.append(box);
  });
  fragment.append(metrics);
  const callout=el('div','callout');
  append(callout,el('strong','','Desired state, not live traffic'),el('p','muted','Counts reflect locally persisted control-plane records. Gateway handshakes, session metrics and effective VPN permissions cannot be inferred from this dashboard.'));
  fragment.append(card('System posture',callout,'ALPHA / NOT ENFORCED'));
  fragment.append(card('Recent administrative actions',table(['Time','Actor','Action','Target'],data.recent_audit,
    [(x)=>formatDate(x.at),(x)=>mono(x.actor_id),(x)=>x.action,(x)=>mono(x.target)],
    'Administrative actions will appear here after a change.'),'Most recent first'));
  return fragment;
}
async function renderUsers() {
  const data=await request('/api/v1/users'); inventory.users=data.items;
  return card('User directory',table(['User','Email','Status','Max devices','Expires','Actions'],data.items,[
    (u)=>u.name,(u)=>u.email||'—',(u)=>badge(u.status),(u)=>u.max_devices,
    (u)=>formatDate(u.expires_at),
    (u)=>permittedToWrite()?actions(action(u.status==='active'?'Suspend':'Activate',async()=>{
      if (!confirm('Change user status? This does NOT immediately disconnect WireGuard peers.'))return;
      await mutate('/api/v1/users/'+encodeURIComponent(u.id),{status:u.status==='active'?'suspended':'active'},'PATCH');
    },u.status==='active')):'—'
  ],'Create a user to start managing device allowances.'),data.items.length+' users');
}
async function renderDevices() {
  const [devices,users]=await Promise.all([request('/api/v1/devices'),request('/api/v1/users')]);
  inventory.devices=devices.items;inventory.users=users.items;
  return card('Registered devices',table(['Device','Owner','Platform','Public key','Status','Action'],devices.items,[
    (d)=>d.name,(d)=>users.items.find((u)=>u.id===d.user_id)?.name||d.user_id,
    (d)=>d.platform,(d)=>mono(d.wireguard_public_key.slice(0,18)+'…'),
    (d)=>badge(d.status),
    (d)=>permittedToWrite() && d.status!=='revoked' ? actions(action('Revoke',async()=>{
      if (!confirm('Revoke device in inventory? IMPORTANT: WireGuard gateway synchronization is NOT implemented. This will NOT yet disconnect the device.'))return;
      await mutate('/api/v1/devices/'+encodeURIComponent(d.id),{status:'revoked'},'PATCH');
    },true)):'—'
  ],'Register a device public key; never upload its private key.'),devices.items.length+' devices');
}
async function renderGateways() {
  const data=await request('/api/v1/gateways');inventory.gateways=data.items;
  return card('Gateway inventory',table(['Gateway','Endpoint','Transport','Status','Added','Action'],data.items,[
    (g)=>g.name,(g)=>mono(g.endpoint),(g)=>g.transport,
    (g)=>badge(g.status),(g)=>formatDate(g.created_at),
    (g)=>permittedToWrite()?actions(action(g.status==='enabled'?'Disable':'Enable',async()=>{
      if(!confirm('Update gateway inventory status? Does NOT alter a live gateway.'))return;
      await mutate('/api/v1/gateways/'+encodeURIComponent(g.id),{status:g.status==='enabled'?'disabled':'enabled'},'PATCH');
    },g.status==='enabled')):'—'
  ],'Register your first direct-IP WireGuard gateway.'),data.items.length+' registered');
}
async function renderPolicies() {
  const [policies,users]=await Promise.all([request('/api/v1/policies'),request('/api/v1/users')]);
  inventory.policies=policies.items;inventory.users=users.items;
  return card('Desired client policy',table(['User','Mode','Preferred transport','Allowed transports','DNS','Updated'],policies.items,[
    (p)=>users.items.find((u)=>u.id===p.user_id)?.name||p.user_id,
    (p)=>p.mode,(p)=>p.preferred_transport||'Auto',
    (p)=>(p.allowed_transports||[]).join(', '),(p)=>p.dns_mode,(p)=>formatDate(p.updated_at)
  ],'No policies configured. These do not yet propagate to client devices.'),'Not enforced');
}
async function renderAudit() {
  const data=await request('/api/v1/audit');
  return card('Control-plane audit events',table(['Timestamp','Actor token ID','Action','Target'],data.items,[
    (x)=>formatDate(x.at),(x)=>mono(x.actor_id),(x)=>x.action,(x)=>mono(x.target)
  ],'The audit log has no recorded administrative changes.'),'Last '+data.items.length+' of '+data.retained_total);
}
async function renderTokens() {
  const data=await request('/api/v1/tokens');inventory.tokens=data.items;
  return card('Administrative access',table(['Name','Role','Created','Expiry','Status','Action'],data.items,[
    (t)=>t.name,(t)=>badge(t.role),(t)=>formatDate(t.created_at),
    (t)=>formatDate(t.expires_at),(t)=>badge(t.revoked?'revoked':'active'),
    (t)=>!t.revoked?actions(action('Revoke',async()=>{
      if (!confirm('Revoke this control-plane token? This cannot be undone.'))return;
      await mutate('/api/v1/tokens/'+encodeURIComponent(t.id)+'/revoke',{},'POST');
    },true)):'—'
  ],'The bootstrap owner token appears here. Create short-lived role tokens as needed.'),'Raw secrets shown once');
}
async function mutate(path, body, method='POST') {
  try {
    await request(path,{method,body});
    toast('Control-plane change saved and audited.');
    await load(view);
  } catch (err) { toast(err.message,true); }
}

function openForm(title, fields, submit) {
  const dialog=$('#modal');
  $('#modal-title').textContent=title;
  $('#modal-error').textContent='';
  $('#modal-form button[type="submit"]').textContent='Save changes';
  const body=$('#modal-fields');body.replaceChildren();
  const controls={};
  for(const field of fields){
    const wrapper=el('div','field');
    const label=el('label','',field.label);
    const control=field.options ? el('select') : el('input');
    control.id='field-'+field.name;
    label.htmlFor=control.id;
    if(field.options) {
      field.options.forEach(([value,text])=>{
        const opt=el('option','',text);opt.value=value;control.append(opt);
      });
    } else {control.type=field.type||'text';control.placeholder=field.placeholder||'';}
    if(field.required)control.required=true;
    if(field.value!==undefined)control.value=String(field.value);
    if(field.min!==undefined)control.min=field.min;
    if(field.max!==undefined)control.max=field.max;
    controls[field.name]=control;
    append(wrapper,label,control);
    if(field.help)wrapper.append(el('small','',field.help));
    body.append(wrapper);
  }
  $('#modal-form').onsubmit=async(e)=>{
    e.preventDefault();$('#modal-error').textContent='';
    const data=Object.fromEntries(Object.entries(controls).map(([name,node])=>[name,node.type==='number'?Number(node.value):node.value.trim()]));
    try {
      const result=await submit(data);
      dialog.close();
      toast('Saved. Audit event recorded.');
      await load(view);
      if(result && result.token) showIssuedToken(result.token);
    } catch(err) {$('#modal-error').textContent=err.message;}
  };
  dialog.showModal();
}
$('#modal-close').addEventListener('click',()=>$('#modal').close());
$('#modal-cancel').addEventListener('click',()=>$('#modal').close());

function newUser() {
  openForm('Create user',[
    {name:'name',label:'Display name',required:true,placeholder:'Alex Smith'},
    {name:'email',label:'Email (optional)',type:'email',placeholder:'user@example.com'},
    {name:'max_devices',label:'Maximum active devices',type:'number',required:true,min:1,max:20,value:3},
    {name:'expires_at',label:'Expiration (optional, RFC3339)',placeholder:'2027-01-01T00:00:00Z'}
  ],(data)=>request('/api/v1/users',{method:'POST',body:data}));
}
async function newDevice() {
  const users=await request('/api/v1/users');
  const options=users.items.filter((u)=>u.status==='active').map((u)=>[u.id,u.name]);
  if(!options.length){toast('Create an active user first.',true);return;}
  openForm('Register device',[
    {name:'user_id',label:'Owner',options,required:true},
    {name:'name',label:'Device label',required:true,placeholder:'Windows laptop'},
    {name:'platform',label:'Platform',options:[['windows','Windows'],['android','Android'],['linux','Linux'],['macos','macOS'],['ios','iOS']]},
    {name:'wireguard_public_key',label:'WireGuard public key',required:true,placeholder:'Base64 public key',help:'Only the public key. Never paste a device private key.'}
  ],(data)=>request('/api/v1/devices',{method:'POST',body:data}));
}
function newGateway() {
  openForm('Register gateway',[
    {name:'name',label:'Gateway name',required:true,placeholder:'Primary gateway'},
    {name:'endpoint',label:'Direct IP:port',required:true,placeholder:'203.0.113.10:51820',help:'IP addresses only in this alpha; no DNS/domain required.'},
    {name:'transport',label:'Transport',options:[['wireguard','WireGuard (current alpha)']]}
  ],(data)=>request('/api/v1/gateways',{method:'POST',body:data}));
}
async function newPolicy() {
  const users=await request('/api/v1/users');
  const options=users.items.map((u)=>[u.id,u.name]);
  if(!options.length){toast('Create a user first.',true);return;}
  openForm('Set desired policy',[
    {name:'user_id',label:'User',options},
    {name:'mode',label:'Path selection',options:[['automatic','Automatic'],['preferred','Preferred'],['manual','Manual']]},
    {name:'preferred_transport',label:'Preferred transport',options:[['wireguard','WireGuard']]},
    {name:'allowed',label:'Allowed transports',value:'wireguard',required:true,help:'Comma-separated names: wireguard, quic, https-relay. Only WireGuard is planned for the first live gateway.'},
    {name:'dns_mode',label:'DNS policy',options:[['tunnel','Tunnel only (desired)']]}
  ],(data)=>{
    const body={mode:data.mode,preferred_transport:data.mode==='automatic'?'':data.preferred_transport,allowed_transports:data.allowed.split(',').map((x)=>x.trim()).filter(Boolean),dns_mode:data.dns_mode};
    return request('/api/v1/policies/'+encodeURIComponent(data.user_id),{method:'PUT',body});
  });
}
function newToken() {
  openForm('Issue scoped API token',[
    {name:'name',label:'Token label',required:true,placeholder:'Temporary auditor token'},
    {name:'role',label:'Role',options:[['auditor','Auditor (read only)'],['operator','Operator (read only in alpha)'],['admin','Admin (inventory write)'],['owner','Owner (tokens + all)']]},
    {name:'expires_hours',label:'Lifetime in hours',type:'number',min:1,max:2160,value:24,required:true}
  ],(data)=>request('/api/v1/tokens',{method:'POST',body:data}));
}
function showIssuedToken(secret) {
  const dialog=$('#modal');
  $('#modal-title').textContent='Save this token now';
  const wrapper=$('#modal-fields');wrapper.replaceChildren();
  wrapper.append(el('p','muted','The secret is returned only once. Do not put it in chat, browser storage or screenshots.'));
  wrapper.append(el('div','token-code',secret));
  const button=el('button','button','Copy token');
  button.type='button';
  button.addEventListener('click',async()=>{
    try {await navigator.clipboard.writeText(secret);toast('Token copied. Store it securely.');}
    catch(_){toast('Clipboard unavailable; manually copy token.',true);}
  });
  wrapper.append(button);
  $('#modal-form').onsubmit=(e)=>{e.preventDefault();dialog.close();};
  $('#modal-form button[type="submit"]').textContent='Done';
  dialog.showModal();
}
