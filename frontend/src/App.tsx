import { useState, useEffect, useRef } from "react";
import "./App.css";
import {
  Login,
  Logout,
  GetStatus,
  GetPlatform,
  GetLicenseInfo,
  PrepareLogin,
  Stop,
  StartWorker,
  StopWorker,
  FindRCS,
} from "../wailsjs/go/main/App";

type Page = "home" | "main" | "sec";

const MAIN_STATUS: Record<string, { label: string; detail: string }> = {
  logged_in: { label: "Ready", detail: "Click Start to begin" },
  login_phase: { label: "Initializing", detail: "Starting session..." },
  running: { label: "Active", detail: "Session is running" },
};

const SEC_STATUS: Record<string, { label: string; detail: string }> = {
  logged_in: { label: "Standby", detail: "Click Start to begin guardian" },
  monitoring: { label: "Guardian Active", detail: "Watching for game process & keeping client alive" },
};

function isMainActive(s: string) {
  return ["login_phase", "running"].includes(s);
}
function isSecActive(s: string) {
  return ["monitoring"].includes(s);
}

/* ─── Icons ─── */
const IconMonitor = () => (
  <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
    <rect x="2" y="3" width="20" height="14" rx="2" />
    <path d="M8 21h8M12 17v4" />
  </svg>
);

const IconShield = () => (
  <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
    <path d="M12 2 3 7v5c0 5.5 3.8 9.3 9 11 5.2-1.7 9-5.5 9-11V7L12 2z" />
  </svg>
);

const IconBack = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M19 12H5M12 19l-7-7 7-7" />
  </svg>
);

const IconChevron = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M9 18l6-6-6-6" />
  </svg>
);

const IconDiamond = ({ size = 12 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor">
    <polygon points="12,2 22,12 12,22 2,12" />
  </svg>
);

function App() {
  const [page, setPage] = useState<Page>("home");
  const [platform, setPlatform] = useState("");
  const [status, setStatus] = useState("disconnected");
  const [username, setUsername] = useState(() => localStorage.getItem("nrx_user") || "");
  const [password, setPassword] = useState(() => localStorage.getItem("nrx_pass") || "");
  const [rcsPath, setRcsPath] = useState("");
  const [error, setError] = useState("");
  const [license, setLicense] = useState<{ has_license: boolean; expires_at: string } | null>(null);
  const pollRef = useRef<number | null>(null);
  const prevStatusRef = useRef(status);

  useEffect(() => {
    GetPlatform().then(setPlatform);
    FindRCS().then((p) => { if (p) setRcsPath(p); });
    pollRef.current = window.setInterval(async () => {
      try { setStatus(await GetStatus()); } catch {}
    }, 1000);
    return () => { if (pollRef.current) clearInterval(pollRef.current); };
  }, []);

  // Auto-navigate: when active status starts, go to that page; when it ends, go home
  useEffect(() => {
    const prev = prevStatusRef.current;
    prevStatusRef.current = status;

    if (isMainActive(status) && page === "home") setPage("main");
    if (isSecActive(status) && page === "home") setPage("sec");

    // If we were active and now idle, go back to home
    if (status === "logged_in" && (isMainActive(prev) || isSecActive(prev))) {
      setPage("home");
    }
  }, [status]);

  const handleLogin = async () => {
    setError("");
    const res = await Login(username, password);
    if (res === "ok") {
      localStorage.setItem("nrx_user", username);
      localStorage.setItem("nrx_pass", password);
      setLicense((await GetLicenseInfo()) as any);
    } else {
      setError(res.includes("too many devices") ? "Account already active on 2 devices." : res);
    }
  };

  const handleLogout = async () => {
    await Logout();
    localStorage.removeItem("nrx_user");
    localStorage.removeItem("nrx_pass");
    setUsername(""); setPassword(""); setLicense(null); setError(""); setPage("home");
  };

  const handleStartMain = async () => {
    setError("");
    const res = await PrepareLogin(rcsPath);
    if (res !== "ok") setError(res);
  };

  const handleStop = async () => {
    setError("");
    const res = await Stop();
    if (res !== "ok" && res !== "nothing to stop") setError(res);
  };

  const handleStartWorker = async () => {
    setError("");
    const res = await StartWorker();
    if (res !== "ok") setError(res);
  };

  const handleStopWorker = async () => {
    setError("");
    const res = await StopWorker();
    if (res !== "ok" && res !== "worker not running") setError(res);
  };

  /* ─── LOGIN SCREEN ─── */
  if (status === "disconnected" || status === "connecting") {
    return (
      <div className="app screen-login">
        <div className="mesh-bg" />
        <div className="login-panel">
          <div className="brand">
            <span className="brand-mark"><IconDiamond size={13} /></span>
            <span className="brand-wordmark">NRX</span>
          </div>
          <p className="login-eyebrow">Premium Access</p>
          <div className="login-fields">
            <input className="field" type="text" placeholder="Username" value={username}
              onChange={(e) => setUsername(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleLogin()} />
            <input className="field" type="password" placeholder="Password" value={password}
              onChange={(e) => setPassword(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleLogin()} />
            <button className="btn-signin" onClick={handleLogin}
              disabled={status === "connecting" || !username || !password}>
              {status === "connecting"
                ? <span className="btn-inner"><span className="spinner-xs" />Connecting</span>
                : "Sign In"}
            </button>
          </div>
          {error && <div className="error">{error}</div>}
          <p className="login-footnote">End-to-end encrypted</p>
        </div>
      </div>
    );
  }

  const mainOn = isMainActive(status);
  const secOn = isSecActive(status);
  const idle = status === "logged_in";

  /* ─── HOME PAGE ─── */
  if (page === "home") {
    return (
      <div className="app screen-main">
        <div className="mesh-bg" />
        <div className="main-wrap">
          <header className="topbar">
            <div className="topbar-brand">
              <span className="brand-mark brand-mark-sm"><IconDiamond size={10} /></span>
              <span className="brand-wordmark brand-wordmark-sm">NRX</span>
            </div>
            <div className="topbar-end">
              {license?.has_license && (
                <div className="lic-badge"><span className="lic-dot" />Licensed</div>
              )}
              <button className="btn-ghost" onClick={handleLogout}>Sign out</button>
            </div>
          </header>

          {error && <div className="error error-bar">{error}</div>}

          <div className="home-cards">
            {/* Main PC card */}
            <div
              className={`home-card home-card-purple${mainOn ? " home-card--active" : ""}${secOn ? " home-card--dim" : ""}`}
              onClick={() => !secOn && setPage("main")}
            >
              <div className="home-card-icon icon-purple"><IconMonitor /></div>
              <div className="home-card-body">
                <h3 className="home-card-title">Main PC</h3>
                <p className="home-card-sub">{mainOn ? (MAIN_STATUS[status]?.label ?? status) : "Gaming machine"}</p>
              </div>
              <div className="home-card-end">
                {mainOn && <span className="dot dot-purple dot-live" />}
                <span className="home-card-chevron"><IconChevron /></span>
              </div>
            </div>

            {/* Secondary PC card */}
            <div
              className={`home-card home-card-cyan${secOn ? " home-card--active" : ""}${mainOn ? " home-card--dim" : ""}`}
              onClick={() => !mainOn && setPage("sec")}
            >
              <div className="home-card-icon icon-cyan"><IconShield /></div>
              <div className="home-card-body">
                <h3 className="home-card-title">Secondary PC</h3>
                <p className="home-card-sub">{secOn ? (SEC_STATUS[status]?.label ?? status) : "League guardian mode"}</p>
              </div>
              <div className="home-card-end">
                {secOn && <span className="dot dot-cyan dot-live" />}
                <span className="home-card-chevron"><IconChevron /></span>
              </div>
            </div>
          </div>

          <footer className="footer">
            {license
              ? license.has_license
                ? `License active${license.expires_at ? " \u00b7 " + license.expires_at : " \u00b7 Lifetime"}`
                : "No active license"
              : ""}
          </footer>
        </div>
      </div>
    );
  }

  /* ─── MAIN PC PAGE ─── */
  if (page === "main") {
    const st = MAIN_STATUS[status] ?? MAIN_STATUS["logged_in"]!;
    const active = isMainActive(status);
    return (
      <div className="app screen-main">
        <div className="mesh-bg" />
        <div className="main-wrap">
          <header className="topbar">
            <button className="btn-back" onClick={() => { setError(""); setPage("home"); }}>
              <IconBack /> Back
            </button>
            <div className="topbar-end">
              <button className="btn-ghost" onClick={handleLogout}>Sign out</button>
            </div>
          </header>

          {error && <div className="error error-bar">{error}</div>}

          <div className="mode-content">
            {/* Status orb */}
            <div className={`status-orb orb-purple${active ? " orb--active" : ""}`}>
              <div className="orb-icon"><IconMonitor /></div>
              {active && <div className="orb-ring" />}
            </div>

            <h2 className="mode-label">{st.label}</h2>
            <p className="mode-detail">{st.detail}</p>

            {/* Detail rows */}
            {active && (
              <div className="detail-panel">
                <div className="detail-row">
                  <span className="detail-key">Riot Client</span>
                  <span className="detail-val val-on">{status === "login_phase" ? "Launching..." : "Running"}</span>
                </div>
                <div className="detail-row">
                  <span className="detail-key">Session</span>
                  <span className="detail-val val-on">Protected</span>
                </div>
              </div>
            )}

            {/* Action */}
            <div className="mode-action">
              {idle && platform === "windows" && (
                <button className="btn-big btn-big-purple" onClick={handleStartMain}>Start Bypass</button>
              )}
              {idle && platform !== "windows" && (
                <p className="mode-hint">Main PC mode is Windows only</p>
              )}
              {active && (
                <button className="btn-big btn-big-stop" onClick={handleStop}>Stop</button>
              )}
            </div>
          </div>

          <footer className="footer">Main PC Mode</footer>
        </div>
      </div>
    );
  }

  /* ─── SECONDARY PC PAGE ─── */
  if (page === "sec") {
    const st = SEC_STATUS[status] ?? SEC_STATUS["logged_in"]!;
    const active = isSecActive(status);
    return (
      <div className="app screen-main">
        <div className="mesh-bg" />
        <div className="main-wrap">
          <header className="topbar">
            <button className="btn-back" onClick={() => { setError(""); setPage("home"); }}>
              <IconBack /> Back
            </button>
            <div className="topbar-end">
              <button className="btn-ghost" onClick={handleLogout}>Sign out</button>
            </div>
          </header>

          {error && <div className="error error-bar">{error}</div>}

          <div className="mode-content">
            {/* Status orb */}
            <div className={`status-orb orb-cyan${active ? " orb--active" : ""}`}>
              <div className="orb-icon"><IconShield /></div>
              {active && <div className="orb-ring orb-ring-cyan" />}
            </div>

            <h2 className="mode-label">{st.label}</h2>
            <p className="mode-detail">{st.detail}</p>

            {/* Detail rows */}
            {active && (
              <div className="detail-panel">
                <div className="detail-row">
                  <span className="detail-key">League Client</span>
                  <span className="detail-val val-on">Kept Alive</span>
                </div>
                <div className="detail-row">
                  <span className="detail-key">Game Process</span>
                  <span className="detail-val val-on">Auto-Kill</span>
                </div>
                <div className="detail-row">
                  <span className="detail-key">Poll Rate</span>
                  <span className="detail-val">1s</span>
                </div>
              </div>
            )}

            {/* Action */}
            <div className="mode-action">
              {idle && (
                <button className="btn-big btn-big-cyan" onClick={handleStartWorker}>Start Guardian</button>
              )}
              {active && (
                <button className="btn-big btn-big-stop" onClick={handleStopWorker}>Stop</button>
              )}
            </div>
          </div>

          <footer className="footer">Secondary PC Mode</footer>
        </div>
      </div>
    );
  }

  return null;
}

export default App;
