import React, { useState, useEffect, useCallback } from 'react';
import { useDropzone } from 'react-dropzone';
import {
  Shield, Lock, UploadCloud, File, CheckCircle,
  Terminal, Loader2, Database, Download, ArrowLeft
} from 'lucide-react';
import './AdminPortal.css';

<<<<<<< HEAD
const apiBase = import.meta.env.VITE_API_BASE || '/api';
=======
// ─────────────────────────────────────────────────────────────────────────────
// AdminPortal — top-level component managing all portal views
// ─────────────────────────────────────────────────────────────────────────────
>>>>>>> b9beb5fda5b9c42a36ceb65d6ebade5a89a6a3ae

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

<<<<<<< HEAD
  // --- Auth Handlers ---
  const handleLogin = async (e) => {
=======
  const handleLogin = (e) => {
>>>>>>> b9beb5fda5b9c42a36ceb65d6ebade5a89a6a3ae
    e.preventDefault();
    setAuthError('');

    try {
      const resp = await fetch(apiBase + '/login', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ passkey: password })
      });

      if (!resp.ok) {
        throw new Error('Invalid passkey');
      }

      setIsAuthenticated(true);
    } catch (err) {
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
<<<<<<< HEAD
      // Simulated validation check (e.g. limit to 100MB)
      if (file.size > 100 * 1024 * 1024) {
        setFileError('FILE EXCEEDS MAXIMUM ALLOWED SIZE (100MB).');
        setSelectedFile(null);
=======
      if (file.size > 50 * 1024 * 1024) {
        setFileError('FILE EXCEEDS MAXIMUM ALLOWED SIZE (50MB).');
>>>>>>> b9beb5fda5b9c42a36ceb65d6ebade5a89a6a3ae
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
<<<<<<< HEAD
    // Accept standard document/archive formats for a project (added PPT/PPTX)
=======
>>>>>>> b9beb5fda5b9c42a36ceb65d6ebade5a89a6a3ae
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
<<<<<<< HEAD
      'application/vnd.ms-powerpoint': ['.ppt'],
      'application/vnd.openxmlformats-officedocument.presentationml.presentation': ['.pptx']
    }
  });

  // --- Form Handlers ---
  const [artifacts, setArtifacts] = useState([]);
  const [loadingArtifacts, setLoadingArtifacts] = useState(false);
=======
    },
  });

  // ─────────────────────────────────────────────────────────────────────────
  // Form
  // ─────────────────────────────────────────────────────────────────────────
>>>>>>> b9beb5fda5b9c42a36ceb65d6ebade5a89a6a3ae

  const handleInputChange = (e) => {
    const { name, value } = e.target;
    setFormData(prev => ({ ...prev, [name]: value }));
  };

<<<<<<< HEAD
  const fetchArchives = async () => {
    setLoadingArtifacts(true);
    setFileError('');
    try {
      const resp = await fetch(apiBase + '/archives', { credentials: 'include' });
      if (!resp.ok) {
        setFileError('Failed to load artifacts: ' + resp.statusText);
        setArtifacts([]);
        setLoadingArtifacts(false);
        return;
      }
      const data = await resp.json();
      setArtifacts(data);
    } catch (err) {
      console.error(err);
      setFileError('Network error when fetching artifacts.');
    }
    setLoadingArtifacts(false);
  };

  const handleLogout = async () => {
    try {
      await fetch(apiBase + '/logout', {
        method: 'POST',
        credentials: 'include'
      });
    } catch (err) {
      console.error('Logout failed', err);
    }

    setIsAuthenticated(false);
    setPassword('');
    setArtifacts([]);
    setSelectedFile(null);
    setPublishSuccess(false);
    setFileError('');
  };

  const handleDelete = async (filename) => {
    if (!confirm('Delete artifact ' + filename + '? This cannot be undone.')) return;
    try {
      const resp = await fetch(apiBase + '/archive?filename=' + encodeURIComponent(filename), {
        method: 'DELETE',
        credentials: 'include'
      });
      if (!resp.ok) { const txt = await resp.text(); setFileError('Delete failed: ' + txt); return; }
      fetchArchives();
    } catch (err) { console.error(err); setFileError('Network error deleting artifact.'); }
  };

  const handlePublish = async () => {
    if (!selectedFile || !formData.title || !formData.version) {
      return;
    }

    setIsPublishing(true);
    setFileError('');

    try {
      const fd = new FormData();
      fd.append('file', selectedFile);
      fd.append('title', formData.title);
      fd.append('version', formData.version);
      fd.append('date', formData.date);
      fd.append('summary', formData.summary);

      const resp = await fetch(apiBase + '/upload', {
        method: 'POST',
        credentials: 'include',
        body: fd
      });

      if (!resp.ok) {
        const txt = await resp.text();
        setFileError(`UPLOAD FAILED: ${txt}`);
        setIsPublishing(false);
        return;
      }

      setIsPublishing(false);
      setPublishSuccess(true);
    } catch (err) {
      console.error('Upload error', err);
      setFileError('NETWORK ERROR: Unable to reach upload endpoint.');
=======
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
>>>>>>> b9beb5fda5b9c42a36ceb65d6ebade5a89a6a3ae
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

<<<<<<< HEAD
        <div className="dashboard-container">
          <div style={{display:'flex', justifyContent:'space-between', gap:'0.75rem', marginBottom:'1rem', flexWrap:'wrap'}}>
            <button className="cyber-button" onClick={fetchArchives} type="button">Load Artifacts</button>
            <button className="cyber-button" onClick={()=>{setArtifacts([]);}} type="button" style={{background:'#2a2a2a'}}>Clear</button>
            <button className="cyber-button" onClick={handleLogout} type="button" style={{background:'#2a2a2a'}}>Sign Out</button>
          </div>

          {/* Dropzone Area */}
          <div 
            {...getRootProps()} 
            className={`dropzone-area ${isDragActive ? 'active' : ''} ${selectedFile ? 'has-file' : ''}`}
=======
        {/* ── Tab switcher ─────────────────────────────────────────────── */}
        <div className="tab-bar">
          <button
            className={`tab-btn ${activeView === 'upload' ? 'active' : ''}`}
            onClick={() => setActiveView('upload')}
>>>>>>> b9beb5fda5b9c42a36ceb65d6ebade5a89a6a3ae
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
<<<<<<< HEAD
                <div className="dropzone-subtext">SUPPORTED FORMATS: .PDF, .PPT, .PPTX, .ZIP, .MD, .TXT</div>
=======
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
>>>>>>> b9beb5fda5b9c42a36ceb65d6ebade5a89a6a3ae
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
<<<<<<< HEAD
            </>
          )}

          {/* Artifacts Manager (Admin) */}
          <div style={{marginTop: '2rem'}}>
            <h3 style={{marginBottom:'0.5rem'}}>Artifacts Manager</h3>
            {loadingArtifacts ? <div>Loading artifacts...</div> : (
              artifacts && artifacts.length>0 ? (
                <div style={{display:'flex',flexDirection:'column',gap:'0.5rem'}}>
                  {artifacts.map(a=> (
                    <div key={a.filename} style={{display:'flex',justifyContent:'space-between',alignItems:'center',padding:'0.5rem',background:'rgba(10,10,10,0.4)',border:'1px solid #1a1a1a'}}>
                      <div>
                        <div style={{fontWeight:700,color:'#ffd700'}}>{a.title || a.originalName}</div>
                        <div style={{fontSize:'0.9rem',color:'#cfcfcf'}}>{a.summary || ''}</div>
                        <div style={{fontSize:'0.8rem',color:'#9a9a9a'}}>{'Uploaded: '+ new Date(a.uploadedAt).toLocaleString() + ' • Version: ' + (a.version||'-')}</div>
                      </div>
                      <div style={{display:'flex',gap:'0.5rem'}}>
                        <a href={a.url} target="_blank" rel="noreferrer" className="cyber-button" style={{background:'#ffd700',color:'#050505',textDecoration:'none'}}>Download</a>
                        <button className="cyber-button" onClick={()=>handleDelete(a.filename)} style={{background:'#8b0000'}}>Delete</button>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="value">No artifacts uploaded yet.</div>
              )
            )}
          </div>
        </div>
=======
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

>>>>>>> b9beb5fda5b9c42a36ceb65d6ebade5a89a6a3ae
      </div>
    </div>
  );
};

export default AdminPortal;
