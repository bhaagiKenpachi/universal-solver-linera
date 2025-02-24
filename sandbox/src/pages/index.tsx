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

export default function Home() {
  const [githubUrl, setGithubUrl] = useState<string>("");
  const [files, setFiles] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

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
      // const files = (event.target as HTMLInputElement).files;
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


        const command = sandbox.shells.run(`rustup target add wasm32-unknown-unknown && cargo build --release --target wasm32-unknown-unknown`);
        command.onOutput((output) => {
          console.log(output);
        });

        // Wait for the dev server port to open
        const portInfo = await sandbox.ports.waitForPort(3001);
        console.log(`Dev server is ready at: ${portInfo.getPreviewUrl()}`);
      // }

    };
    // Open the folder dialog
    // folderInput.dispatchEvent(new MouseEvent('click'));
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


  return (
    <>
      <input 
        type="text" 
        placeholder="Enter GitHub URL" 
        value={githubUrl} 
        onChange={(e) => setGithubUrl(e.target.value)} 
      />
      {files.length ? (
          <ul>
            {files.map(file => (
                <li key={file.sha}>
                  <a href={file.html_url} target="_blank" rel="noopener noreferrer">
                    {file.name}
                  </a>
                  <span> ({file.type})</span>
                </li>
            ))}
          </ul>
      ) : (
          <p>Loading files...</p>
      )}
      {/*<button onClick={ async () => await fetchGithubFiles(githubUrl)}>Submit</button>*/}
      <button onClick={uploadToCodeSandbox}>Start CodeSandbox</button>
      
    </>
  );
}
