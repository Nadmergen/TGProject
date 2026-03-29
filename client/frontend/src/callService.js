import { writable, get } from 'svelte/store';
import { recipient, currentUser } from './stores';
import { API_URL } from './config';

export const callState = writable({
  active: false,
  incoming: false,
  callType: 'voice', // voice|video
  peerUserId: null,
  peerUsername: '',
  muted: false,
  cameraOff: false,
});

let ws = null;
let pc = null;
let localStream = null;
let remoteStream = null;

export function setCallWebSocket(socket) {
  ws = socket;
}

export function getRemoteStream() {
  return remoteStream;
}

export function getLocalStream() {
  return localStream;
}

function getIceServers() {
  // VITE_ICE_SERVERS expects JSON like: [{"urls":["stun:stun.l.google.com:19302"]},{"urls":["turn:..."],"username":"u","credential":"p"}]
  const raw = import.meta.env?.VITE_ICE_SERVERS;
  if (raw) {
    try { return JSON.parse(raw); } catch (_) {}
  }
  return [{ urls: ['stun:stun.l.google.com:19302'] }];
}

async function ensurePeerConnection(callType) {
  if (pc) return pc;
  pc = new RTCPeerConnection({ iceServers: getIceServers() });
  remoteStream = new MediaStream();

  pc.ontrack = (ev) => {
    for (const t of ev.streams[0].getTracks()) remoteStream.addTrack(t);
  };

  pc.onicecandidate = (ev) => {
    if (ev.candidate && ws) {
      const st = get(callState);
      ws.send(JSON.stringify({
        event: 'call_ice',
        recipient_id: st.peerUserId,
        candidate: ev.candidate,
        call_type: callType
      }));
    }
  };

  return pc;
}

async function getUserMediaFor(callType) {
  const constraints = callType === 'video'
    ? { audio: true, video: { width: { ideal: 1280 }, height: { ideal: 720 } } }
    : { audio: true, video: false };
  return await navigator.mediaDevices.getUserMedia(constraints);
}

export async function startOutgoingCall(callType = 'voice') {
  const target = get(recipient);
  const me = get(currentUser);
  if (!target?.id || !me?.id) return;
  if (!ws) throw new Error('WebSocket not ready');

  callState.set({
    active: true,
    incoming: false,
    callType,
    peerUserId: target.id,
    peerUsername: target.username || '',
    muted: false,
    cameraOff: callType !== 'video',
  });

  localStream = await getUserMediaFor(callType);
  const peer = await ensurePeerConnection(callType);
  for (const track of localStream.getTracks()) peer.addTrack(track, localStream);

  const offer = await peer.createOffer();
  await peer.setLocalDescription(offer);

  ws.send(JSON.stringify({
    event: 'call_offer',
    recipient_id: target.id,
    sdp: offer,
    call_type: callType
  }));
}

export async function acceptIncomingCall() {
  const st = get(callState);
  if (!st.incoming || !ws) return;

  localStream = await getUserMediaFor(st.callType);
  const peer = await ensurePeerConnection(st.callType);
  for (const track of localStream.getTracks()) peer.addTrack(track, localStream);

  callState.update(s => ({ ...s, active: true, incoming: false }));
}

export async function hangup() {
  const st = get(callState);
  if (ws && st.peerUserId) {
    ws.send(JSON.stringify({ event: 'call_hangup', recipient_id: st.peerUserId, call_type: st.callType }));
  }
  cleanup();
}

function cleanup() {
  try { pc?.close(); } catch (_) {}
  pc = null;
  if (localStream) {
    localStream.getTracks().forEach(t => t.stop());
  }
  localStream = null;
  remoteStream = null;
  callState.set({ active: false, incoming: false, callType: 'voice', peerUserId: null, peerUsername: '', muted: false, cameraOff: false });
}

export async function handleSignalingMessage(msg) {
  if (!msg?.event) return;
  if (msg.event === 'call_offer') {
    const nextType = msg.call_type || 'voice';

    // renegotiation while already in call
    const st = get(callState);
    if (st.active && pc) {
      // reflect call type upgrade in UI
      callState.update(s => ({
        ...s,
        callType: nextType,
        cameraOff: nextType !== 'video' ? true : s.cameraOff
      }));

      // if upgrading to video, ensure we have a local video track
      if (nextType === 'video' && localStream && localStream.getVideoTracks().length === 0) {
        try {
          const v = await navigator.mediaDevices.getUserMedia({ video: true, audio: false });
          const vTrack = v.getVideoTracks()[0];
          if (vTrack) {
            localStream.addTrack(vTrack);
            pc.addTrack(vTrack, localStream);
            callState.update(s => ({ ...s, callType: 'video', cameraOff: false }));
          }
        } catch (_) {}
      }

      await pc.setRemoteDescription(new RTCSessionDescription(msg.sdp));
      const answer = await pc.createAnswer();
      await pc.setLocalDescription(answer);
      if (ws && st.peerUserId) {
        ws.send(JSON.stringify({
          event: 'call_answer',
          recipient_id: st.peerUserId,
          sdp: answer,
          call_type: nextType
        }));
      }
      return;
    }

    // incoming call offer (fresh)
    callState.set({
      active: false,
      incoming: true,
      callType: nextType,
      peerUserId: msg.sender_id,
      peerUsername: '',
      muted: false,
      cameraOff: nextType !== 'video',
    });

    const peer = await ensurePeerConnection(nextType);
    await peer.setRemoteDescription(new RTCSessionDescription(msg.sdp));
    return;
  }

  if (!pc) return;

  if (msg.event === 'call_answer') {
    await pc.setRemoteDescription(new RTCSessionDescription(msg.sdp));
  } else if (msg.event === 'call_ice' && msg.candidate) {
    try { await pc.addIceCandidate(new RTCIceCandidate(msg.candidate)); } catch (_) {}
  } else if (msg.event === 'call_hangup') {
    cleanup();
  }
}

export async function sendAnswer() {
  const st = get(callState);
  if (!ws || !pc || !st.peerUserId) return;
  const answer = await pc.createAnswer();
  await pc.setLocalDescription(answer);
  ws.send(JSON.stringify({
    event: 'call_answer',
    recipient_id: st.peerUserId,
    sdp: answer,
    call_type: st.callType
  }));
}

export function toggleMute() {
  if (!localStream) return;
  const enabledNow = localStream.getAudioTracks().some(t => t.enabled);
  localStream.getAudioTracks().forEach(t => t.enabled = !enabledNow);
  callState.update(s => ({ ...s, muted: enabledNow }));
}

export function toggleCamera() {
  if (!localStream) return;
  const tracks = localStream.getVideoTracks();
  if (!tracks.length) return;
  const enabledNow = tracks.some(t => t.enabled);
  tracks.forEach(t => t.enabled = !enabledNow);
  callState.update(s => ({ ...s, cameraOff: enabledNow }));
}

export async function enableVideo() {
  const st = get(callState);
  if (!st.active || !pc || !ws || !st.peerUserId) return;
  if (!localStream) return;
  if (localStream.getVideoTracks().length > 0) {
    callState.update(s => ({ ...s, callType: 'video', cameraOff: false }));
    return;
  }
  const v = await navigator.mediaDevices.getUserMedia({ video: true, audio: false });
  const vTrack = v.getVideoTracks()[0];
  if (!vTrack) return;
  localStream.addTrack(vTrack);
  pc.addTrack(vTrack, localStream);
  callState.update(s => ({ ...s, callType: 'video', cameraOff: false }));

  const offer = await pc.createOffer();
  await pc.setLocalDescription(offer);
  ws.send(JSON.stringify({
    event: 'call_offer',
    recipient_id: st.peerUserId,
    sdp: offer,
    call_type: 'video'
  }));
}

