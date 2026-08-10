import React, { useState, useCallback } from 'react';
import { useDropzone } from 'react-dropzone';
import { Shield, Lock, UploadCloud, File, CheckCircle, Terminal, Loader2 } from 'lucide-react';
import './AdminPortal.css';

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
  const handleLogin = (e) => {
    e.preventDefault();
    if (password === 'admin') {
      setIsAuthenticated(true);
      setAuthError('');
    } else {
      setAuthError('ACCESS DENIED. INVALID CREDENTIALS.');
    }
  };

  // --- Dropzone Handlers ---
  const onDrop = useCallback(acceptedFiles => {
    setFileError('');
    if (acceptedFiles.length > 0) {
      const file = acceptedFiles[0];
      // Simulated validation check (e.g. limit to 50MB)
      if (file.size > 50 * 1024 * 1024) {
        setFileError('FILE EXCEEDS MAXIMUM ALLOWED SIZE (50MB).');
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
    // Accept standard document/archive formats for a project
    accept: {
      'application/pdf': ['.pdf'],
      'application/zip': ['.zip'],
      'application/x-rar-compressed': ['.rar'],
      'text/markdown': ['.md'],
      'text/plain': ['.txt']
    }
  });

  // --- Form Handlers ---
  const handleInputChange = (e) => {
    const { name, value } = e.target;
    setFormData(prev => ({ ...prev, [name]: value }));
  };

  const handlePublish = () => {
    if (!selectedFile || !formData.title || !formData.version) {
      return; // Basic validation
    }
    
    setIsPublishing(true);
    
    // Simulate API Call execution
    setTimeout(() => {
      setIsPublishing(false);
      setPublishSuccess(true);
    }, 2500);
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
                <div className="dropzone-subtext">SUPPORTED FORMATS: .PDF, .ZIP, .MD, .TXT</div>
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
        </div>
      </div>
    </div>
  );
};

export default AdminPortal;
