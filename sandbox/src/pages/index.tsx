import { Geist, Geist_Mono } from "next/font/google";
import { CodeSandbox } from "@codesandbox/sdk";
import { useEffect, useState } from "react";
import LZString from 'lz-string';
import { log } from "console";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

interface ProjectFile {
  content: string;
}

interface ProjectFiles {
  [filePath: string]: ProjectFile;
}

interface Repository {
    id: number;
    name: string;
    full_name: string;
    description: string;
    html_url: string;
    private: boolean;
}

interface RepoContent {
    type: string;
    name: string;
    path: string;
    sha: string;
    size: number;
    url: string;
    download_url: string | null;
}

interface RepoFile {
    path: string;
    content: string;
    type: string;
    size: number;
    sha: string;
}

const API_BASE_URL = 'https://uni-solver.ngrok.io';

export default function Home() {
  const [githubUrl, setGithubUrl] = useState<string>("");
  const [files, setFiles] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [repositories, setRepositories] = useState<Repository[]>([]);
  const [isAuthenticated, setIsAuthenticated] = useState(false);

  const deployBytecode = async (contractWasm: Uint8Array, serviceWasm: Uint8Array) => {
    console.log("Deploying bytecode...");
    console.log(`Contract size: ${contractWasm.length} bytes`);
    console.log(`Service size: ${serviceWasm.length} bytes`);

    try {
      // Create the request body with size headers
      const body = new Uint8Array(contractWasm.length + serviceWasm.length + 100); // Extra space for headers
      let offset = 0;

      // Write contract size header
      const contractSizeStr = `${contractWasm.length}|`;
      const contractSizeBytes = new TextEncoder().encode(contractSizeStr);
      body.set(contractSizeBytes, offset);
      offset += contractSizeBytes.length;

      // Write service size header
      const serviceSizeStr = `${serviceWasm.length}|`;
      const serviceSizeBytes = new TextEncoder().encode(serviceSizeStr);
      body.set(serviceSizeBytes, offset);
      offset += serviceSizeBytes.length;

      // Write contract WASM
      body.set(contractWasm, offset);
      offset += contractWasm.length;

      // Write service WASM
      body.set(serviceWasm, offset);

      // Send the request
      const response = await fetch('http://localhost:3001/deploy_bytecode', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/octet-stream',
        },
        body: body,
      });

      if (!response.ok) {
        const errorData = await response.json();
        console.error('Error deploying bytecode:', errorData.message);
        throw new Error(`Failed to deploy bytecode: ${errorData.message}`);
      }

      const data = await response.json();
      console.log('Successfully deployed bytecode:', data);
      return data.data.bytecodeId;
    } catch (error) {
      console.error('Error in deployBytecode:', error);
      throw error;
    }
  };

  const createApplication = async (bytecodeId: string) => {
    console.log("Creating application with bytecode ID:", bytecodeId);
    
    try {
        const response = await fetch('http://localhost:3001/create_application', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                bytecodeId: bytecodeId,
            }),
        });

        if (!response.ok) {
            const errorData = await response.json();
            console.error('Error creating application:', errorData.message);
            throw new Error(`Failed to create application: ${errorData.message}`);
        }

        const data = await response.json();
        console.log('Successfully created application:', data);
        return data.data.applicationId;
    } catch (error) {
        console.error('Error in createApplication:', error);
        throw error;
    }
  };

  const pushToCodeSandbox = async (files: ProjectFiles) => {
    const sdk = new CodeSandbox(process.env.NEXT_PUBLIC_CODESANDBOX_TOKEN);
    // const sandbox = await sdk.sandbox.create({
    //   template: 'ej14tt'
    // });

    const storedSandboxId = localStorage.getItem('sandboxId');
    let sandbox;
    if (storedSandboxId) {
      // Load the sandbox with the stored ID
      
      sandbox = await sdk.sandbox.open(storedSandboxId);
    } else {
      // Store the sandbox ID in localStorage
      sandbox = await sdk.sandbox.create({
        template: 'rk69p3'
      });

      localStorage.setItem('sandboxId', sandbox.id);
    }

    // const folderInput = document.createElement('input');
    // folderInput.type = 'file';
    // folderInput.webkitdirectory = true; // Allow folder selection

    // folderInput.onchange = async (event) => {
      // const files = (event.target as HTMLIn.putElement).files;
      if (files) {
        for (const [path, { content }] of Object.entries(files)) {

          console.log(path)
          await sandbox.fs.writeTextFile(path, content);
        }

        console.log("files uploaded to sandbox")

        // Listen to setup progress
        sandbox.setup.onSetupProgressUpdate((progress) => {
          console.log(`Setup progress: ${progress.currentStepIndex + 1}/${progress.steps.length}`);
          console.log(`Current step: ${progress.steps[progress.currentStepIndex].name}`);
        });

        // Get current progress
        const progress = await sandbox.setup.getProgress();
        console.log(`Setup state: ${progress.state}`);

      // Wait for setup to finish
        const result = await sandbox.setup.waitForFinish();
        if (result.state === "FINISHED") {
          console.log("Setup completed successfully");
        }

        // Get all tasks
        const tasks = await sandbox.tasks.getTasks();
        // Run all startup tasks
        for (const task of tasks) {
          console.log(`Starting ${task.name}...`);
          sandbox.tasks.runTask(task.id);
        }

    const checkTasksRunning = async () => {
      const tasks = await sandbox.tasks.getTasks();
      const runningTasks = tasks.filter(task => task.state === 'IN_PROGRESS');

      if (runningTasks.length > 0) {
        console.log(`Currently running tasks: ${runningTasks.map(task => task.name).join(', ')}`);
        setTimeout(checkTasksRunning, 5000); // Check again after 5 seconds
      } else {
        console.log("No tasks are currently running.");
      }
    };

    checkTasksRunning();

    };

    const cargoToml = await sandbox.fs.readFile('./Cargo.toml');

    const cargoTomlContent = cargoToml ? new TextDecoder().decode(cargoToml) : '';
    console.log(cargoTomlContent);
    const binNames = cargoTomlContent.match(/\[\[bin\]\][\s\S]*?name\s*=\s*"([^"]+)"/g)?.map(line => {
      const match = line.match(/name\s*=\s*"([^"]+)"/);
      return match ? match[1] : null;
    }).filter(name => name !== null) || [];

    console.log("Extracted bin names:", binNames);
    const contractNames = binNames.filter(bin => /_contract$/.test(bin));
    const serviceNames = binNames.filter(bin => /_service$/.test(bin));

    const _contract = await sandbox.fs.readFile(`./target/wasm32-unknown-unknown/release/${contractNames[0]}.wasm`);
    const _service = await sandbox.fs.readFile(`./target/wasm32-unknown-unknown/release/${serviceNames[0]}.wasm`);
    console.log(_contract)
    console.log(_service)

  const bytecodeId = await deployBytecode(_contract, _service);
  const applicationId = await createApplication(bytecodeId);
  console.log('Application created with ID:', applicationId);

  };

  // Recursively fetch directory contents from GitHub
  async function fetchDirectory(owner: string, repo: string, branch: string): Promise<ProjectFiles> {
    try {
      // Repository details
      const owner = 'bhaagiKenpachi';
      const repo = 'universal-solver-linera';
      const branch = 'main';
      const token = process.env.NEXT_PUBLIC_GITHUB_TOKEN
      console.log(token)
      // Get branch info to retrieve the tree SHA
      const branchRes = await fetch(`https://api.github.com/repos/${owner}/${repo}/branches/${branch}`, {
        headers: {
          'Authorization': `token ${token}`,
          'Accept': 'application/vnd.github.v3+json'
        }
      });
      if (!branchRes.ok) throw new Error(`Failed to fetch branch: ${branchRes.status}`);
      const branchData = await branchRes.json();
      const treeSha = branchData.commit.commit.tree.sha;

      // Get the entire tree recursively
      const treeRes = await fetch(`https://api.github.com/repos/${owner}/${repo}/git/trees/${treeSha}?recursive=1`, {
        headers: {
          'Authorization': `token ${token}`,
          'Accept': 'application/vnd.github.v3+json'
        }
      });
      if (!treeRes.ok) throw new Error(`Failed to fetch tree: ${treeRes.status}`);
      const treeData = await treeRes.json();

      // Filter for files (blobs)
      const fileTree = treeData.tree.filter(item => item.type === 'blob');

      // Build the CodeSandbox files object in the format: { files: { "path/to/file": { content: "..." } } }
      const filesObject = { files: {} };

      // Loop over each file and fetch its content from the GitHub Contents API.
      for (const file of fileTree) {
        // Using the GitHub Contents API to get the file content (Base64 encoded)
        const fileRes = await fetch(`https://api.github.com/repos/${owner}/${repo}/contents/${file.path}?ref=${branch}`, {
          headers: {
            'Authorization': `token ${token}`,
            'Accept': 'application/vnd.github.v3+json'
          }
        });
        if (!fileRes.ok) {
          console.warn(`Failed to fetch ${file.path}: ${fileRes.status}`);
          continue;
        }
        const fileData = await fileRes.json();
        // Decode the Base64 content (the API returns content with line breaks, so remove them)
        const content = atob(fileData.content.replace(/\n/g, ''));
        filesObject.files[file.path] = { content };
      }
      return filesObject.files
    } catch (err) {
      setError(err.message);
    }
  }

  async function uploadToCodeSandbox() {
    setLoading(true);
    setError(null);
    try {
      // Set repository details based on your URL:
      // https://github.com/dojimanetwork/linera-integration-demo/tree/improve-http-request-system-api/examples/universal-solver

      const owner = 'bhaagiKenpachi';
      const repo = 'universal-solver-linera';
      const path = 'examples/universal-solver';
      const branch = 'improve-http-request-system-api';
      const apiUrl = `https://api.github.com/repos/${owner}/${repo}?ref=${branch}`;

      // Recursively fetch files from the given folder
      const files = await fetchDirectory(owner, repo, branch);
      console.log(files)
      await pushToCodeSandbox(files);

    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    // Check if user is authenticated
    const checkAuth = async () => {
      try {
        const response = await fetch(`${API_BASE_URL}/repos`, {
          credentials: 'include',
          headers: {
            'Accept': 'application/json',
          },
        });
        
        if (response.ok) {
          const data = await response.json();
          setRepositories(data.data.repositories);
          setIsAuthenticated(true);
        }
      } catch (error) {
        console.error('Error checking auth:', error);
      }
    };
    checkAuth();
  }, []);

  const handleLogin = () => {
    window.location.href = `${API_BASE_URL}/auth/github`;
  };

  if (!isAuthenticated) {
    return <button onClick={handleLogin}>Login with GitHub</button>;
  }

  return (
    <>
      <GithubAuth />
      {/*<input */}
      {/*  type="text" */}
      {/*  placeholder="Enter GitHub URL" */}
      {/*  value={githubUrl} */}
      {/*  onChange={(e) => setGithubUrl(e.target.value)} */}
      {/*/>*/}
      {/*{files.length ? (*/}
      {/*    <ul>*/}
      {/*      {files.map(file => (*/}
      {/*          <li key={file.sha}>*/}
      {/*            <a href={file.html_url} target="_blank" rel="noopener noreferrer">*/}
      {/*              {file.name}*/}
      {/*            </a>*/}
      {/*            <span> ({file.type})</span>*/}
      {/*          </li>*/}
      {/*      ))}*/}
      {/*    </ul>*/}
      {/*) : (*/}
      {/*    <p>Loading files...</p>*/}
      {/*)}*/}
      {/*/!*<button onClick={ async () => await fetchGithubFiles(githubUrl)}>Submit</button>*!/*/}
      {/*<button onClick={uploadToCodeSandbox}>Start CodeSandbox</button>*/}
      
    </>
  );
}

const RepoFiles = ({ owner, repo }: { owner: string; repo: string }) => {
    const [contents, setContents] = useState<RepoContent[]>([]);
    const [currentPath, setCurrentPath] = useState<string>('');
    const [loading, setLoading] = useState(false);

    const fetchContents = async (path: string = '') => {
        setLoading(true);
        try {
            const response = await fetch(
                `http://localhost:3001/repo/files?owner=${owner}&repo=${repo}&path=${path}`,
                {
                    credentials: 'include',
                    headers: {
                        'Accept': 'application/json',
                    },
                }
            );

            if (!response.ok) {
                throw new Error('Failed to fetch repository contents');
            }

            const data = await response.json();
            setContents(data.data.contents);
            setCurrentPath(path);
        } catch (error) {
            console.error('Error fetching contents:', error);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchContents();
    }, [owner, repo]);

    const handleFileClick = (content: RepoContent) => {
        if (content.type === 'dir') {
            fetchContents(content.path);
        } else {
            window.open(content.download_url!, '_blank');
        }
    };

    const handleBackClick = () => {
        const parentPath = currentPath.split('/').slice(0, -1).join('/');
        fetchContents(parentPath);
    };

    if (loading) {
        return <div>Loading...</div>;
    }

    return (
        <div className="repo-contents">
            {currentPath && (
                <button onClick={handleBackClick} className="back-button">
                    ← Back
                </button>
            )}
            <div className="path-display">
                Current path: /{currentPath}
            </div>
            <div className="contents-list">
                {contents.map(content => (
                    <div
                        key={content.sha}
                        className="content-item"
                        onClick={() => handleFileClick(content)}
                    >
                        <span className="content-icon">
                            {content.type === 'dir' ? '📁' : '📄'}
                        </span>
                        <span className="content-name">{content.name}</span>
                        {content.type === 'file' && (
                            <span className="content-size">
                                {(content.size / 1024).toFixed(1)} KB
                            </span>
                        )}
                    </div>
                ))}
            </div>
        </div>
    );
};

const RepoFileViewer = ({ owner, repo }: { owner: string; repo: string }) => {
    const [files, setFiles] = useState<RepoFile[]>([]);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [selectedFile, setSelectedFile] = useState<RepoFile | null>(null);

    useEffect(() => {
        const fetchAllFiles = async () => {
            setLoading(true);
            setError(null);
            try {
                const response = await fetch(
                    `${API_BASE_URL}/repo/all-files?owner=${owner}&repo=${repo}`,
                    {
                        credentials: 'include',
                        headers: {
                            'Accept': 'application/json',
                        },
                    }
                );

                if (!response.ok) {
                    throw new Error('Failed to fetch repository files');
                }

                const data = await response.json();
                setFiles(data.data.files);
            } catch (error) {
                setError(error.message);
                console.error('Error fetching files:', error);
            } finally {
                setLoading(false);
            }
        };

        fetchAllFiles();
    }, [owner, repo]);

    const handleFileClick = (file: RepoFile) => {
        setSelectedFile(file);
    };

    if (loading) {
        return <div>Loading repository files...</div>;
    }

    if (error) {
        return <div>Error: {error}</div>;
    }

    return (
        <div className="file-viewer">
            <div className="file-list">
                <h3>Repository Files</h3>
                {files.map(file => (
                    <div
                        key={file.sha}
                        className={`file-item ${selectedFile?.path === file.path ? 'selected' : ''}`}
                        onClick={() => handleFileClick(file)}
                    >
                        <span className="file-icon">📄</span>
                        <span className="file-path">{file.path}</span>
                        <span className="file-size">
                            {(file.size / 1024).toFixed(1)} KB
                        </span>
                    </div>
                ))}
            </div>
            {selectedFile && (
                <div className="file-content">
                    <h3>{selectedFile.path}</h3>
                    <pre>
                        <code>{selectedFile.content}</code>
                    </pre>
                </div>
            )}
        </div>
    );
};

const GithubAuth = () => {
  const [repositories, setRepositories] = useState<Repository[]>([]);
  const [isAuthenticated, setIsAuthenticated] = useState(false);

  useEffect(() => {
    const checkAuth = async () => {
      try {
        const response = await fetch(`${API_BASE_URL}/repos`, {
          credentials: 'include',
          headers: {
            'Accept': 'application/json',
          },
        });
        
        if (response.ok) {
          const data = await response.json();
          setRepositories(data.data.repositories);
          setIsAuthenticated(true);
        }
      } catch (error) {
        console.error('Error checking auth:', error);
      }
    };

    // Check for auth success parameter in URL
    const urlParams = new URLSearchParams(window.location.search);
    if (urlParams.get('auth') === 'success') {
      checkAuth();
      window.history.replaceState({}, document.title, window.location.pathname);
    } else {
      checkAuth();
    }
  }, []);

  const handleLogin = () => {
    window.location.href = `${API_BASE_URL}/auth/github`;
  };

  if (!isAuthenticated) {
    return (
      <button onClick={handleLogin} className="login-button">
        Login with GitHub
      </button>
    );
  }

  return (
    <div>
      <h2>Your Repositories</h2>
      <div className="repository-list">
        {repositories.map(repo => (
          <div key={repo.id} className="repository-item">
            <h3>{repo.name}</h3>
            <p>{repo.description}</p>
            <RepoFileViewer owner={repo.owner.login} repo={repo.name} />
            <a href={repo.html_url} target="_blank" rel="noopener noreferrer">
              View on GitHub
            </a>
          </div>
        ))}
      </div>
    </div>
  );
};
