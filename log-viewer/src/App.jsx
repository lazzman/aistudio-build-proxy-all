import { useState, useEffect, useRef } from "react";
import {
  Search,
  Filter,
  AlertCircle,
  Info,
  AlertTriangle,
  Bug,
  RefreshCw,
  Download,
  Trash2,
  Activity,
  Users,
  Database,
  Copy,
  Check,
  ArrowRight,
  Send,
  MessageSquare,
} from "lucide-react";
import "./App.css";

const API_BASE = "http://localhost:5345";

function App() {
  const [logs, setLogs] = useState([]);
  const [filteredLogs, setFilteredLogs] = useState([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [levelFilter, setLevelFilter] = useState("ALL");
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [health, setHealth] = useState(null);
  const [selectedLog, setSelectedLog] = useState(null);
  const [copiedField, setCopiedField] = useState(null);
  const logsEndRef = useRef(null);

  const scrollToBottom = () => {
    logsEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  const fetchLogs = async () => {
    try {
      const response = await fetch(`${API_BASE}/api/logs`);
      const data = await response.json();
      setLogs(data.logs || []);
    } catch (error) {
      console.error("Failed to fetch logs:", error);
    }
  };

  const fetchHealth = async () => {
    try {
      const response = await fetch(`${API_BASE}/api/health`);
      const data = await response.json();
      setHealth(data);
    } catch (error) {
      console.error("Failed to fetch health:", error);
    }
  };

  useEffect(() => {
    fetchLogs();
    fetchHealth();

    if (autoRefresh) {
      const interval = setInterval(() => {
        fetchLogs();
        fetchHealth();
      }, 2000);
      return () => clearInterval(interval);
    }
  }, [autoRefresh]);

  useEffect(() => {
    let filtered = logs;

    if (levelFilter !== "ALL") {
      filtered = filtered.filter((log) => log.level === levelFilter);
    }

    if (searchTerm) {
      const term = searchTerm.toLowerCase();
      filtered = filtered.filter(
        (log) =>
          log.message.toLowerCase().includes(term) ||
          JSON.stringify(log.data || {})
            .toLowerCase()
            .includes(term),
      );
    }

    setFilteredLogs(filtered);
  }, [logs, searchTerm, levelFilter]);

  useEffect(() => {
    if (autoRefresh) {
      scrollToBottom();
    }
  }, [filteredLogs, autoRefresh]);

  const clearLogs = () => {
    if (
      confirm(
        "Are you sure you want to clear all logs? This will refresh from server.",
      )
    ) {
      fetchLogs();
    }
  };

  const downloadLogs = () => {
    const dataStr = JSON.stringify(logs, null, 2);
    const blob = new Blob([dataStr], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `logs-${new Date().toISOString()}.json`;
    link.click();
  };

  const getLevelIcon = (level) => {
    switch (level) {
      case "ERROR":
        return <AlertCircle className="icon error-icon" />;
      case "WARN":
        return <AlertTriangle className="icon warn-icon" />;
      case "DEBUG":
        return <Bug className="icon debug-icon" />;
      default:
        return <Info className="icon info-icon" />;
    }
  };

  const getLevelClass = (level) => {
    return `log-entry ${level.toLowerCase()}`;
  };

  const formatTimestamp = (timestamp) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString("en-US", {
      hour12: false,
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      fractionalSecondDigits: 3,
    });
  };

  const formatJSON = (data) => {
    if (!data || Object.keys(data).length === 0) return null;
    return JSON.stringify(data, null, 2);
  };

  const copyToClipboard = (text, fieldName) => {
    navigator.clipboard.writeText(text).then(() => {
      setCopiedField(fieldName);
      setTimeout(() => setCopiedField(null), 2000);
    });
  };

  const getLogTypeIcon = (message) => {
    if (message.includes("[REQUEST"))
      return <Send className="flow-icon request" />;
    if (message.includes("[RESPONSE"))
      return <MessageSquare className="flow-icon response" />;
    if (message.includes("[STREAM"))
      return <Activity className="flow-icon stream" />;
    if (message.includes("[ERROR"))
      return <AlertCircle className="flow-icon error" />;
    return null;
  };

  const getFlowDirection = (message) => {
    if (message.includes("[REQUEST")) return "Client → Gemini API";
    if (message.includes("[RESPONSE")) return "Gemini API → Client";
    if (message.includes("[STREAM")) return "Gemini API ⟿ Client";
    return null;
  };

  const logCount = {
    total: logs.length,
    error: logs.filter((l) => l.level === "ERROR").length,
    warn: logs.filter((l) => l.level === "WARN").length,
    info: logs.filter((l) => l.level === "INFO").length,
    debug: logs.filter((l) => l.level === "DEBUG").length,
  };

  return (
    <div className="app">
      <header className="header">
        <div className="header-title">
          <Activity className="header-icon" />
          <h1>AI Studio Proxy - Log Viewer</h1>
        </div>

        {health && (
          <div className="health-stats">
            <div className="health-stat">
              <Users size={16} />
              <span>{health.active_users} users</span>
            </div>
            <div className="health-stat">
              <Database size={16} />
              <span>{health.active_connections} connections</span>
            </div>
            <div className="health-stat">
              <Activity size={16} />
              <span className="status-dot"></span>
              <span>{health.status}</span>
            </div>
          </div>
        )}
      </header>

      <div className="controls">
        <div className="search-bar">
          <Search size={18} />
          <input
            type="text"
            placeholder="Search logs..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
          />
        </div>

        <div className="filter-buttons">
          <button
            className={`filter-btn ${levelFilter === "ALL" ? "active" : ""}`}
            onClick={() => setLevelFilter("ALL")}
          >
            ALL ({logCount.total})
          </button>
          <button
            className={`filter-btn error ${levelFilter === "ERROR" ? "active" : ""}`}
            onClick={() => setLevelFilter("ERROR")}
          >
            <AlertCircle size={14} />
            ERROR ({logCount.error})
          </button>
          <button
            className={`filter-btn warn ${levelFilter === "WARN" ? "active" : ""}`}
            onClick={() => setLevelFilter("WARN")}
          >
            <AlertTriangle size={14} />
            WARN ({logCount.warn})
          </button>
          <button
            className={`filter-btn info ${levelFilter === "INFO" ? "active" : ""}`}
            onClick={() => setLevelFilter("INFO")}
          >
            <Info size={14} />
            INFO ({logCount.info})
          </button>
          <button
            className={`filter-btn debug ${levelFilter === "DEBUG" ? "active" : ""}`}
            onClick={() => setLevelFilter("DEBUG")}
          >
            <Bug size={14} />
            DEBUG ({logCount.debug})
          </button>
        </div>

        <div className="action-buttons">
          <button
            className={`action-btn ${autoRefresh ? "active" : ""}`}
            onClick={() => setAutoRefresh(!autoRefresh)}
            title="Toggle auto-refresh"
          >
            <RefreshCw size={18} className={autoRefresh ? "spinning" : ""} />
          </button>
          <button
            className="action-btn"
            onClick={downloadLogs}
            title="Download logs"
          >
            <Download size={18} />
          </button>
          <button
            className="action-btn"
            onClick={clearLogs}
            title="Refresh logs"
          >
            <Trash2 size={18} />
          </button>
        </div>
      </div>

      <div className="logs-container">
        <div className="logs-list">
          {filteredLogs.length === 0 ? (
            <div className="empty-state">
              <Info size={48} />
              <p>No logs to display</p>
              <small>Logs will appear here as requests are processed</small>
            </div>
          ) : (
            filteredLogs.map((log, index) => (
              <div
                key={index}
                className={getLevelClass(log.level)}
                onClick={() => setSelectedLog(log)}
              >
                <div className="log-header">
                  <div className="log-time-group">
                    {getLogTypeIcon(log.message)}
                    <span className="log-time">
                      {formatTimestamp(log.timestamp)}
                    </span>
                  </div>
                  <span className="log-level">
                    {getLevelIcon(log.level)}
                    {log.level}
                  </span>
                </div>
                {getFlowDirection(log.message) && (
                  <div className="flow-direction">
                    {getFlowDirection(log.message)}
                  </div>
                )}
                <div className="log-message">{log.message}</div>
                {log.data && Object.keys(log.data).length > 0 && (
                  <div className="log-data-preview">
                    <Filter size={12} />
                    <span>{Object.keys(log.data).length} data fields</span>
                  </div>
                )}
              </div>
            ))
          )}
          <div ref={logsEndRef} />
        </div>

        {selectedLog && (
          <div className="log-detail">
            <div className="detail-header">
              <div className="detail-header-left">
                {getLogTypeIcon(selectedLog.message)}
                <h3>Log Details</h3>
              </div>
              <button onClick={() => setSelectedLog(null)}>×</button>
            </div>
            <div className="detail-content">
              <div className="detail-section">
                <div className="detail-label-row">
                  <label>Timestamp:</label>
                  <button
                    className="copy-btn"
                    onClick={() =>
                      copyToClipboard(
                        new Date(selectedLog.timestamp).toISOString(),
                        "timestamp",
                      )
                    }
                    title="Copy timestamp"
                  >
                    {copiedField === "timestamp" ? (
                      <Check size={14} />
                    ) : (
                      <Copy size={14} />
                    )}
                  </button>
                </div>
                <div className="detail-value">
                  {new Date(selectedLog.timestamp).toISOString()}
                </div>
              </div>

              {getFlowDirection(selectedLog.message) && (
                <div className="detail-section">
                  <label>Flow Direction:</label>
                  <div className="detail-flow">
                    <ArrowRight className="flow-arrow" />
                    <span>{getFlowDirection(selectedLog.message)}</span>
                  </div>
                </div>
              )}

              <div className="detail-section">
                <label>Level:</label>
                <div
                  className={`detail-value level-${selectedLog.level.toLowerCase()}`}
                >
                  {getLevelIcon(selectedLog.level)}
                  {selectedLog.level}
                </div>
              </div>

              <div className="detail-section">
                <div className="detail-label-row">
                  <label>Message:</label>
                  <button
                    className="copy-btn"
                    onClick={() =>
                      copyToClipboard(selectedLog.message, "message")
                    }
                    title="Copy message"
                  >
                    {copiedField === "message" ? (
                      <Check size={14} />
                    ) : (
                      <Copy size={14} />
                    )}
                  </button>
                </div>
                <div className="detail-value">{selectedLog.message}</div>
              </div>

              {selectedLog.data && selectedLog.data.request_id && (
                <div className="detail-section">
                  <div className="detail-label-row">
                    <label>Request ID:</label>
                    <button
                      className="copy-btn"
                      onClick={() =>
                        copyToClipboard(
                          selectedLog.data.request_id,
                          "request_id",
                        )
                      }
                      title="Copy request ID"
                    >
                      {copiedField === "request_id" ? (
                        <Check size={14} />
                      ) : (
                        <Copy size={14} />
                      )}
                    </button>
                  </div>
                  <div className="detail-value mono">
                    {selectedLog.data.request_id}
                  </div>
                </div>
              )}

              {selectedLog.data && selectedLog.data.url && (
                <div className="detail-section">
                  <div className="detail-label-row">
                    <label>URL:</label>
                    <button
                      className="copy-btn"
                      onClick={() =>
                        copyToClipboard(selectedLog.data.url, "url")
                      }
                      title="Copy URL"
                    >
                      {copiedField === "url" ? (
                        <Check size={14} />
                      ) : (
                        <Copy size={14} />
                      )}
                    </button>
                  </div>
                  <div className="detail-value mono">
                    {selectedLog.data.url}
                  </div>
                </div>
              )}

              {selectedLog.data && selectedLog.data.method && (
                <div className="detail-section">
                  <label>HTTP Method:</label>
                  <div className="detail-value method">
                    {selectedLog.data.method}
                  </div>
                </div>
              )}

              {selectedLog.data && selectedLog.data.status && (
                <div className="detail-section">
                  <label>Status Code:</label>
                  <div
                    className={`detail-value status ${selectedLog.data.status >= 400 ? "error" : "success"}`}
                  >
                    {selectedLog.data.status}
                  </div>
                </div>
              )}

              {selectedLog.data && selectedLog.data.headers && (
                <div className="detail-section">
                  <div className="detail-label-row">
                    <label>Headers:</label>
                    <button
                      className="copy-btn"
                      onClick={() =>
                        copyToClipboard(
                          formatJSON(selectedLog.data.headers),
                          "headers",
                        )
                      }
                      title="Copy headers"
                    >
                      {copiedField === "headers" ? (
                        <Check size={14} />
                      ) : (
                        <Copy size={14} />
                      )}
                    </button>
                  </div>
                  <pre className="detail-json">
                    {formatJSON(selectedLog.data.headers)}
                  </pre>
                </div>
              )}

              {selectedLog.data && selectedLog.data.body && (
                <div className="detail-section">
                  <div className="detail-label-row">
                    <label>Request/Response Body:</label>
                    <button
                      className="copy-btn"
                      onClick={() =>
                        copyToClipboard(
                          typeof selectedLog.data.body === "string"
                            ? selectedLog.data.body
                            : formatJSON(selectedLog.data.body),
                          "body",
                        )
                      }
                      title="Copy body"
                    >
                      {copiedField === "body" ? (
                        <Check size={14} />
                      ) : (
                        <Copy size={14} />
                      )}
                    </button>
                  </div>
                  <pre className="detail-json body">
                    {typeof selectedLog.data.body === "string"
                      ? selectedLog.data.body
                      : formatJSON(selectedLog.data.body)}
                  </pre>
                </div>
              )}

              {selectedLog.data && selectedLog.data.error && (
                <div className="detail-section error-section">
                  <div className="detail-label-row">
                    <label>Error Details:</label>
                    <button
                      className="copy-btn"
                      onClick={() =>
                        copyToClipboard(selectedLog.data.error, "error")
                      }
                      title="Copy error"
                    >
                      {copiedField === "error" ? (
                        <Check size={14} />
                      ) : (
                        <Copy size={14} />
                      )}
                    </button>
                  </div>
                  <div className="detail-value error">
                    {selectedLog.data.error}
                  </div>
                </div>
              )}

              {selectedLog.data && selectedLog.data.transformations && (
                <div className="detail-section">
                  <label>
                    Tool Transformations (
                    {selectedLog.data.transformations.length} tools):
                  </label>
                  <pre className="detail-json">
                    {formatJSON(selectedLog.data.transformations)}
                  </pre>
                </div>
              )}

              {selectedLog.data &&
                selectedLog.data.all_removed_fields_detail && (
                  <div className="detail-section">
                    <div className="detail-label-row">
                      <label>
                        Removed Fields Detail (
                        {
                          Object.keys(
                            selectedLog.data.all_removed_fields_detail,
                          ).length
                        }{" "}
                        fields):
                      </label>
                      <button
                        className="copy-btn"
                        onClick={() =>
                          copyToClipboard(
                            formatJSON(
                              selectedLog.data.all_removed_fields_detail,
                            ),
                            "removed_fields",
                          )
                        }
                        title="Copy removed fields"
                      >
                        {copiedField === "removed_fields" ? (
                          <Check size={14} />
                        ) : (
                          <Copy size={14} />
                        )}
                      </button>
                    </div>
                    <pre className="detail-json">
                      {formatJSON(selectedLog.data.all_removed_fields_detail)}
                    </pre>
                  </div>
                )}

              {selectedLog.data && selectedLog.data.removed_field && (
                <div className="detail-section">
                  <label>Removed Field:</label>
                  <div className="detail-value">
                    {selectedLog.data.removed_field}
                  </div>
                  {selectedLog.data.removed_value && (
                    <>
                      <label>Removed Value:</label>
                      <pre className="detail-json">
                        {typeof selectedLog.data.removed_value === "string"
                          ? selectedLog.data.removed_value
                          : formatJSON(selectedLog.data.removed_value)}
                      </pre>
                    </>
                  )}
                </div>
              )}

              {selectedLog.data && selectedLog.data.original_field && (
                <div className="detail-section">
                  <label>Field Conversion:</label>
                  <div className="detail-value">
                    {selectedLog.data.original_field}:{" "}
                    {JSON.stringify(selectedLog.data.original_value)} →{" "}
                    {selectedLog.data.converted_field}:{" "}
                    {JSON.stringify(selectedLog.data.converted_value)}
                  </div>
                </div>
              )}

              {selectedLog.data && selectedLog.data.payload && (
                <div className="detail-section">
                  <div className="detail-label-row">
                    <label>Full Payload:</label>
                    <button
                      className="copy-btn"
                      onClick={() =>
                        copyToClipboard(
                          formatJSON(selectedLog.data.payload),
                          "payload",
                        )
                      }
                      title="Copy payload"
                    >
                      {copiedField === "payload" ? (
                        <Check size={14} />
                      ) : (
                        <Copy size={14} />
                      )}
                    </button>
                  </div>
                  <pre className="detail-json">
                    {formatJSON(selectedLog.data.payload)}
                  </pre>
                </div>
              )}

              {selectedLog.data && Object.keys(selectedLog.data).length > 0 && (
                <div className="detail-section">
                  <div className="detail-label-row">
                    <label>All Data (JSON):</label>
                    <button
                      className="copy-btn primary"
                      onClick={() =>
                        copyToClipboard(formatJSON(selectedLog.data), "all")
                      }
                      title="Copy all data"
                    >
                      {copiedField === "all" ? (
                        <Check size={14} />
                      ) : (
                        <Copy size={14} />
                      )}
                      <span>Copy All</span>
                    </button>
                  </div>
                  <pre className="detail-json">
                    {formatJSON(selectedLog.data)}
                  </pre>
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

export default App;
