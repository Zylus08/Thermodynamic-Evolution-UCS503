import React, { useState, useCallback } from 'react';
import { useDropzone } from 'react-dropzone';
import { Shield, Lock, UploadCloud, File, CheckCircle, Terminal, Loader2 } from 'lucide-react';
import './AdminPortal.css';

const apiBase = import.meta.env.VITE_API_BASE || '/api';

const AdminPortal = () => {
  // Authentication State
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [password, setPassword] = useState('');
  const [authError, setAuthError] = useState('');

  // Dropzone / File State
  const [selectedFile, setSelectedFile] = useState(null);
  const [fileError, setFileError] = useState('');

  // Metadata Form State
  const [formData, setFormData] = useState({
    title: '',
    version: '',
    date: new Date().toISOString().split('T')[0],
    summary: ''
  });

  // Publish Execution State
  const [isPublishing, setIsPublishing] = useState(false);
  const [publishSuccess, setPublishSuccess] = useState(false);

  // --- Auth Handlers ---
  const handleLogin = async (e) => {
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

  // --- Dropzone Handlers ---
  const onDrop = useCallback(acceptedFiles => {
    setFileError('');
    if (acceptedFiles.length > 0) {
      const file = acceptedFiles[0];
      // Simulated validation check (e.g. limit to 100MB)
      if (file.size > 100 * 1024 * 1024) {
        setFileError('FILE EXCEEDS MAXIMUM ALLOWED SIZE (100MB).');
        setSelectedFile(null);
        return;
      }
      setSelectedFile(file);
      // Auto-fill title based on filename
      setFormData(prev => ({
        ...prev,
        title: file.name.split('.')[0].replace(/[-_]/g, ' ')
      }));
    }
  }, []);

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    multiple: false,
    // Accept standard document/archive formats for a project (added PPT/PPTX)
    accept: {
      'application/pdf': ['.pdf'],
      'application/zip': ['.zip'],
      'application/x-rar-compressed': ['.rar'],
      'text/markdown': ['.md'],
      'text/plain': ['.txt'],
      'application/vnd.ms-powerpoint': ['.ppt'],
      'application/vnd.openxmlformats-officedocument.presentationml.presentation': ['.pptx']
    }
  });

  // --- Form Handlers ---
  const [artifacts, setArtifacts] = useState([]);
  const [loadingArtifacts, setLoadingArtifacts] = useState(false);

  const handleInputChange = (e) => {
    const { name, value } = e.target;
    setFormData(prev => ({ ...prev, [name]: value }));
  };

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
      setIsPublishing(false);
    }
  };

  // --- Render Auth Screen ---
  if (!isAuthenticated) {
    return (
      <div className="admin-portal-wrapper">
        <div className="portal-container">
          <div className="portal-header">
            <div className="header-title">
              <Shield size={20} />
              SYS.SECURE // INSTRUCTOR ACCESS
            </div>
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
              <button type="submit" className="cyber-button">
                INITIALIZE SESSION
              </button>
            </form>
          </div>
        </div>
      </div>
    );
  }

  // --- Render Success State ---
  if (publishSuccess) {
    return (
      <div className="admin-portal-wrapper">
        <div className="portal-container">
          <div className="portal-header">
            <div className="header-title">
              <Terminal size={20} />
              SYS.ARCHIVE // UPLOAD COMPLETE
            </div>
          </div>
          
          <div className="success-container">
            <CheckCircle size={64} className="success-icon" />
            <h1 className="success-title">Archive Generated</h1>
            <p className="success-text">
              Target file [ <strong>{selectedFile?.name}</strong> ] has been successfully encrypted and archived.<br />
              Version {formData.version} is now active.<br />
              Previous iterations remain intact in the cold storage matrix.
            </p>
            <button className="cyber-button" onClick={() => window.location.reload()}>
              RETURN TO DASHBOARD
            </button>
          </div>
        </div>
      </div>
    );
  }

  // --- Render Main Dashboard ---
  return (
    <div className="admin-portal-wrapper">
      <div className="portal-container">
        <div className="portal-header">
          <div className="header-title">
            <Terminal size={20} />
            SYS.ARCHIVE // INSTRUCTOR CONSOLE
          </div>
          <div className="status-indicator">
            <div className="status-dot"></div>
            AUTHORIZED : ADMIN
          </div>
        </div>

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
                  {isDragActive ? "DEPLOY FILE HERE" : "DRAG & DROP ARCHIVE OR CLICK TO BROWSE"}
                </div>
                <div className="dropzone-subtext">SUPPORTED FORMATS: .PDF, .PPT, .PPTX, .ZIP, .MD, .TXT</div>
              </>
            )}
          </div>
          {fileError && <div className="error-text" style={{textAlign: 'center', marginBottom: '2rem'}}>{fileError}</div>}

          {/* Metadata Form */}
          {selectedFile && (
            <>
              <div className="metadata-form">
                <div className="input-group">
                  <label className="input-label">Document Title</label>
                  <input 
                    type="text" 
                    name="title"
                    className="cyber-input" 
                    value={formData.title}
                    onChange={handleInputChange}
                    placeholder="Enter document title"
                  />
                </div>
                
                <div className="input-group">
                  <label className="input-label">Presentation Version</label>
                  <input 
                    type="text" 
                    name="version"
                    className="cyber-input" 
                    value={formData.version}
                    onChange={handleInputChange}
                    placeholder="e.g. v1.2.0"
                  />
                </div>
                
                <div className="input-group">
                  <label className="input-label">Archive Date</label>
                  <input 
                    type="date" 
                    name="date"
                    className="cyber-input" 
                    value={formData.date}
                    onChange={handleInputChange}
                  />
                </div>

                <div className="input-group full-width">
                  <label className="input-label">Change Summary</label>
                  <textarea 
                    name="summary"
                    className="cyber-input cyber-textarea" 
                    value={formData.summary}
                    onChange={handleInputChange}
                    placeholder="Brief description of modifications..."
                  ></textarea>
                </div>
              </div>

              {/* Publish Action */}
              <button 
                className="cyber-button" 
                style={{width: '100%'}}
                onClick={handlePublish}
                disabled={!formData.title || !formData.version || isPublishing}
              >
                {isPublishing ? (
                  <><Loader2 className="spinner" size={20} /> EXECUTING ARCHIVE SEQUENCE...</>
                ) : (
                  "PUBLISH TO ARCHIVE"
                )}
              </button>
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
      </div>
    </div>
  );
};

export default AdminPortal;
