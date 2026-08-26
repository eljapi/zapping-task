const PLAYLIST = "/playlist.m3u8";
const video = document.getElementById("video");
const errorBox = document.getElementById("error");
let reloads = 0;

function set(id, value) {
  document.getElementById(id).textContent = value;
}

function fail(message) {
  errorBox.textContent = message;
  errorBox.style.display = "block";
}

if (Hls.isSupported()) {
  const hls = new Hls({ lowLatencyMode: false });
  hls.loadSource(PLAYLIST);
  hls.attachMedia(video);

  hls.on(Hls.Events.LEVEL_UPDATED, (_, data) => {
    const details = data.details;
    set("seq", details.startSN);
    set("count", details.fragments.length);
    set("target", details.targetduration + "s");
    set("reloads", ++reloads);
  });

  hls.on(Hls.Events.ERROR, (_, data) => {
    if (data.fatal) {
      fail(data.type + ": " + data.details);
    }
  });
} else if (video.canPlayType("application/vnd.apple.mpegurl")) {
  video.src = PLAYLIST;
} else {
  fail("HLS is not supported in this browser.");
}
