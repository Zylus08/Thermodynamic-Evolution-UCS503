import React, { useState, useEffect, useCallback } from 'react';
import { useDropzone } from 'react-dropzone';
import {
  Shield, Lock, UploadCloud, File, CheckCircle,
  Terminal, Loader2, Database, Download, ArrowLeft
} from 'lucide-react';
import './AdminPortal.css';

// ─────────────────────────────────────────────────────────────────────────────
// AdminPortal — top-level component managing all portal views
// ─────────────────────────────────────────────────────────────────────────────

const AdminPortal = () => {

  // ── Auth ──────────────────────────────────────────────────────────────────
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [password, setPassword] = useState('');
  const [authError, setAuthError] = useState('');

  // ── Active view: 'upload' | 'archive' ────────────────────────────────────
  const [activeView, setActiveView] = useState('upload');

  // ── Dropzone / file state ─────────────────────────────────────────────────
  const [selectedFile, setSelectedFile] = useState(null);
  const [fileError, setFileError] = useState('');

  // ── Metadata form ─────────────────────────────────────────────────────────
  const [formData, setFormData] = useState({
    title: '',
    version: '',
    date: new Date().toISOString().split('T')[0],
    summary: '',
  });

  // ── Publish state ─────────────────────────────────────────────────────────
  const [isPublishing, setIsPublishing] = useState(false);
  const [publishSuccess, setPublishSuccess] = useState(false);
  const [publishError, setPublishError] = useState('');
  // The full Deliverable object returned by the backend after a successful upload
  const [lastDeliverable, setLastDeliverable] = useState(null);

  // ── Archive / deliverables list ───────────────────────────────────────────
  const [deliverables, setDeliverables] = useState([]);
  const [archiveLoading, setArchiveLoading] = useState(false);
  const [archiveError, setArchiveError] = useState('');

  // ─────────────────────────────────────────────────────────────────────────
  // Auth
  // ─────────────────────────────────────────────────────────────────────────

  const handleLogin = (e) => {
    e.preventDefault();
    if (password === 'admin') {
      setIsAuthenticated(true);
      setAuthError('');
    } else {
      setAuthError('ACCESS DENIED. INVALID CREDENTIALS.');
    }
  };

  // ─────────────────────────────────────────────────────────────────────────
  // Dropzone
  // react-dropzone requires BOTH the correct MIME type key AND the extension
  // in the accept map. Providing both .ppt and .pptx keys fixes the rejection.
  // ─────────────────────────────────────────────────────────────────────────

  const onDrop = useCallback((acceptedFiles, rejectedFiles) => {
    setFileError('');

    // Surface a useful message when the browser rejects a file type
    if (rejectedFiles.length > 0) {
      const reason = rejectedFiles[0].errors?.[0]?.message || 'Invalid file type.';
      setFileError(`FILE REJECTED: ${reason.toUpperCase()}`);
      return;
    }

    if (acceptedFiles.length > 0) {
      const file = acceptedFiles[0];
      if (file.size > 50 * 1024 * 1024) {
        setFileError('FILE EXCEEDS MAXIMUM ALLOWED SIZE (50MB).');
        return;
      }
      setSelectedFile(file);
      // Auto-fill the title from the filename (strip extension, clean separators)
      setFormData(prev => ({
        ...prev,
        title: file.name.replace(/\.[^.]+$/, '').replace(/[-_]/g, ' '),
      }));
    }
  }, []);

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    multiple: false,
    accept: {
      // ── PowerPoint (both legacy .ppt and modern .pptx) ───────────────────
      // react-dropzone validates by MIME type; the browser may report either
      // MIME for a .pptx, so we list all known variants to be safe.
      'application/vnd.ms-powerpoint': ['.ppt'],
      'application/vnd.openxmlformats-officedocument.presentationml.presentation': ['.pptx'],
      // ── Other document formats ────────────────────────────────────────────
      'application/pdf': ['.pdf'],
      'application/zip': ['.zip'],
      'text/markdown': ['.md'],
      'text/plain': ['.txt'],
    },
  });

  // ─────────────────────────────────────────────────────────────────────────
  // Form
  // ─────────────────────────────────────────────────────────────────────────

  const handleInputChange = (e) => {
    const { name, value } = e.target;
    setFormData(prev => ({ ...prev, [name]: value }));
  };

  // ─────────────────────────────────────────────────────────────────────────
  // Publish — real multipart/form-data POST to Go backend
  // ─────────────────────────────────────────────────────────────────────────

  const handlePublish = async () => {
    if (!selectedFile || !formData.title || !formData.version) return;

    setIsPublishing(true);
    setPublishError('');

    try {
      // Build multipart payload — do NOT set Content-Type, let the browser add
      // the correct boundary string automatically.
      const body = new FormData();
      body.append('file', selectedFile);
      body.append('title', formData.title);
      body.append('version', formData.version);
      body.append('date', formData.date);
      body.append('summary', formData.summary);

      // Vite proxies /api/* → http://localhost:8080/* (strips /api prefix)
      const response = await fetch('/api/upload', { method: 'POST', body });
      const json = await response.json();

      if (!response.ok || !json.success) {
        throw new Error(json.message || `HTTP ${response.status}`);
      }

      // Backend now returns the full Deliverable object under json.data
      setLastDeliverable(json.data);
      setPublishSuccess(true);

    } catch (err) {
      console.error('[UPLOAD ERROR]', err);
      setPublishError(`UPLOAD FAILED: ${err.message}`);
    } finally {
      setIsPublishing(false);
    }
  };

  // ─────────────────────────────────────────────────────────────────────────
  // Archive — fetch all deliverables from Go backend
  // ─────────────────────────────────────────────────────────────────────────

  const fetchDeliverables = useCallback(async () => {
    setArchiveLoading(true);
    setArchiveError('');
    try {
      const response = await fetch('/api/deliverables');
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const data = await response.json();
      // Backend always returns an array (never null)
      setDeliverables(Array.isArray(data) ? data : []);
    } catch (err) {
      console.error('[ARCHIVE ERROR]', err);
      setArchiveError(`FAILED TO LOAD ARCHIVE: ${err.message}`);
    } finally {
      setArchiveLoading(false);
    }
  }, []);

  // Reload archive data whenever the user switches to the archive view
  useEffect(() => {
    if (activeView === 'archive' && isAuthenticated) {
      fetchDeliverables();
    }
  }, [activeView, isAuthenticated, fetchDeliverables]);

  // ─────────────────────────────────────────────────────────────────────────
  // Reset upload flow
  // ─────────────────────────────────────────────────────────────────────────

  const resetUpload = () => {
    setSelectedFile(null);
    setFileError('');
    setPublishError('');
    setPublishSuccess(false);
    setLastDeliverable(null);
    setFormData({
      title: '',
      version: '',
      date: new Date().toISOString().split('T')[0],
      summary: '',
    });
  };

  // ─────────────────────────────────────────────────────────────────────────
  // RENDER: Login screen
  // ─────────────────────────────────────────────────────────────────────────

  if (!isAuthenticated) {
    return (
      <div className="admin-portal-wrapper">
        <div className="portal-container">
          <div className="portal-header">
            <div className="header-title"><Shield size={20} /> SYS.SECURE // INSTRUCTOR ACCESS</div>
            <div className="status-indicator">
              <div className="status-dot"></div>
              ENCRYPTED CONNECTION
            </div>
          </div>

          <div className="auth-container">
            <Lock size={48} className="auth-icon" />
            <h1 className="auth-title">Authentication Required</h1>
            <p className="auth-subtitle">ENTER CREDENTIALS TO ACCESS ARCHIVE SYSTEM</p>

            <form className="auth-form" onSubmit={handleLogin}>
              <div className="input-group">
                <label className="input-label">Passkey</label>
                <input
                  type="password"
                  className="cyber-input"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="• • • • • • • •"
                  autoFocus
                />
                {authError && <div className="error-text">{authError}</div>}
              </div>
              <button type="submit" className="cyber-button">INITIALIZE SESSION</button>
            </form>
          </div>
        </div>
      </div>
    );
  }

  // ─────────────────────────────────────────────────────────────────────────
  // RENDER: Success screen
  // ─────────────────────────────────────────────────────────────────────────

  if (publishSuccess && lastDeliverable) {
    return (
      <div className="admin-portal-wrapper">
        <div className="portal-container">
          <div className="portal-header">
            <div className="header-title"><Terminal size={20} /> SYS.ARCHIVE // UPLOAD COMPLETE</div>
          </div>

          <div className="success-container">
            <CheckCircle size={64} className="success-icon" />
            <h1 className="success-title">Archive Generated</h1>
            <p className="success-text">
              {/* Dynamic values from the backend response — no hardcoding */}
              File&nbsp;<strong>[ {lastDeliverable.filename} ]</strong>&nbsp;committed to storage matrix.<br />
              Version&nbsp;<strong>{lastDeliverable.version}</strong>&nbsp;is now active.<br />
              Previous iterations remain intact in cold storage.
            </p>

            {/* Direct link served by Go's static file handler */}
            <a
              href={lastDeliverable.file_url}
              target="_blank"
              rel="noopener noreferrer"
              className="cyber-button"
              style={{ marginBottom: '1rem', textDecoration: 'none' }}
            >
              ACCESS ARCHIVED FILE ↗
            </a>

            <div style={{ display: 'flex', gap: '1rem', width: '100%' }}>
              <button className="cyber-button" style={{ flex: 1 }} onClick={resetUpload}>
                NEW UPLOAD
              </button>
              <button
                className="cyber-button"
                style={{ flex: 1 }}
                onClick={() => { resetUpload(); setActiveView('archive'); }}
              >
                VIEW ARCHIVE
              </button>
            </div>
          </div>
        </div>
      </div>
    );
  }

  // ─────────────────────────────────────────────────────────────────────────
  // RENDER: Main dashboard (Upload tab + Archive tab)
  // ─────────────────────────────────────────────────────────────────────────

  return (
    <div className="admin-portal-wrapper">
      <div className="portal-container">

        {/* ── Header with tab navigation ──────────────────────────────── */}
        <div className="portal-header">
          <div className="header-title"><Terminal size={20} /> SYS.ARCHIVE // INSTRUCTOR CONSOLE</div>
          <div className="status-indicator">
            <div className="status-dot"></div>
            AUTHORIZED : ADMIN
          </div>
        </div>

        {/* ── Tab switcher ─────────────────────────────────────────────── */}
        <div className="tab-bar">
          <button
            className={`tab-btn ${activeView === 'upload' ? 'active' : ''}`}
            onClick={() => setActiveView('upload')}
          >
            <UploadCloud size={14} /> UPLOAD
          </button>
          <button
            className={`tab-btn ${activeView === 'archive' ? 'active' : ''}`}
            onClick={() => setActiveView('archive')}
          >
            <Database size={14} /> VIEW ARCHIVE
          </button>
        </div>

        {/* ══════════════════════════════════════════════════════════════ */}
        {/* UPLOAD VIEW                                                   */}
        {/* ══════════════════════════════════════════════════════════════ */}
        {activeView === 'upload' && (
          <div className="dashboard-container">

            {/* Dropzone */}
            <div
              {...getRootProps()}
              className={`dropzone-area ${isDragActive ? 'active' : ''} ${selectedFile ? 'has-file' : ''}`}
            >
              <input {...getInputProps()} />
              {selectedFile ? (
                <div className="file-info">
                  <File size={32} />
                  <span>{selectedFile.name}</span>
                </div>
              ) : (
                <>
                  <UploadCloud size={48} className="upload-icon" />
                  <div className="dropzone-text">
                    {isDragActive ? 'DEPLOY FILE HERE' : 'DRAG & DROP ARCHIVE OR CLICK TO BROWSE'}
                  </div>
                  <div className="dropzone-subtext">SUPPORTED: .PPTX · .PPT · .PDF · .ZIP · .MD · .TXT</div>
                </>
              )}
            </div>

            {fileError && (
              <div className="error-text" style={{ textAlign: 'center', marginBottom: '1.5rem' }}>
                {fileError}
              </div>
            )}

            {/* Metadata form — only visible once a file is selected */}
            {selectedFile && (
              <>
                <div className="metadata-form">
                  <div className="input-group">
                    <label className="input-label">Document Title</label>
                    <input type="text" name="title" className="cyber-input"
                      value={formData.title} onChange={handleInputChange}
                      placeholder="Enter document title" />
                  </div>

                  <div className="input-group">
                    <label className="input-label">Presentation Version</label>
                    <input type="text" name="version" className="cyber-input"
                      value={formData.version} onChange={handleInputChange}
                      placeholder="e.g. v1.2.0" />
                  </div>

                  <div className="input-group">
                    <label className="input-label">Archive Date</label>
                    <input type="date" name="date" className="cyber-input"
                      value={formData.date} onChange={handleInputChange} />
                  </div>

                  <div className="input-group full-width">
                    <label className="input-label">Change Summary</label>
                    <textarea name="summary" className="cyber-input cyber-textarea"
                      value={formData.summary} onChange={handleInputChange}
                      placeholder="Brief description of modifications..." />
                  </div>
                </div>

                {publishError && (
                  <div className="error-text" style={{ textAlign: 'center', marginBottom: '1rem' }}>
                    {publishError}
                  </div>
                )}

                <button
                  className="cyber-button"
                  style={{ width: '100%' }}
                  onClick={handlePublish}
                  disabled={!formData.title || !formData.version || isPublishing}
                >
                  {isPublishing
                    ? <><Loader2 className="spinner" size={20} /> TRANSMITTING TO BACKEND...</>
                    : 'PUBLISH TO ARCHIVE'}
                </button>
              </>
            )}
          </div>
        )}

        {/* ══════════════════════════════════════════════════════════════ */}
        {/* ARCHIVE VIEW                                                  */}
        {/* ══════════════════════════════════════════════════════════════ */}
        {activeView === 'archive' && (
          <div className="dashboard-container">

            <div className="archive-header-row">
              <span className="archive-title">DELIVERABLES ARCHIVE</span>
              <button className="cyber-button-sm" onClick={fetchDeliverables} disabled={archiveLoading}>
                {archiveLoading ? <Loader2 className="spinner" size={14} /> : '⟳ REFRESH'}
              </button>
            </div>

            {archiveError && (
              <div className="error-text" style={{ marginBottom: '1rem' }}>{archiveError}</div>
            )}

            {archiveLoading && (
              <div className="archive-loading">
                <Loader2 className="spinner" size={28} style={{ color: 'var(--gold)' }} />
                <span>QUERYING ARCHIVE MATRIX...</span>
              </div>
            )}

            {!archiveLoading && deliverables.length === 0 && (
              <div className="archive-empty">
                <Database size={40} style={{ color: 'var(--text-muted)', marginBottom: '0.75rem' }} />
                <p>NO RECORDS FOUND IN ARCHIVE.</p>
                <p style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                  Upload a deliverable to begin.
                </p>
              </div>
            )}

            {!archiveLoading && deliverables.length > 0 && (
              <div className="archive-table-wrapper">
                <table className="archive-table">
                  <thead>
                    <tr>
                      <th>#</th>
                      <th>TITLE</th>
                      <th>VERSION</th>
                      <th>DATE</th>
                      <th>CHANGE SUMMARY</th>
                      <th>FILE</th>
                    </tr>
                  </thead>
                  <tbody>
                    {/* Reverse so newest entries appear at top */}
                    {[...deliverables].reverse().map((d) => (
                      <tr key={d.id}>
                        <td className="archive-id">{String(d.id).padStart(3, '0')}</td>
                        <td className="archive-title-cell">{d.title}</td>
                        <td className="archive-version">{d.version}</td>
                        <td className="archive-date">{d.date}</td>
                        <td className="archive-summary">{d.summary || '—'}</td>
                        <td>
                          <a
                            href={d.file_url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="archive-download-link"
                            title={d.filename}
                          >
                            <Download size={14} />
                            DOWNLOAD
                          </a>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}

      </div>
    </div>
  );
};

export default AdminPortal;
