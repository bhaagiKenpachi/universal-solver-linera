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

  const fetchGithubFiles = async (url: string) => {

    const owner = 'dojimanetwork';
    const repo = 'linera-integration-demo';
    const path = 'examples/universal-solver';
    const branch = 'improve-http-request-system-api';
    const apiUrl = `https://api.github.com/repos/${owner}/${repo}/contents/${path}?ref=${branch}`;

    try {
      const response = await fetch(apiUrl);
      if (!response.ok) {
        throw new Error(`GitHub API returned status ${response.status}`);
      }
      const data = await response.json();
      setFiles(data);
      console.log("Fetched files:", data);
      // You can process the files as needed here
    } catch (error) {
      console.error("Error:", (error as Error).message);
    }
  };

  const pushToCodeSandbox = async (files: ProjectFiles) => {
    const sdk = new CodeSandbox("csb_v1_Wl0js3HQFQFRwznrIRib7jZKbidc8Ec6dN2SCLXCMAY");
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
        template: 'ej14tt'
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
          await sandbox.fs.writeTextFile(path, content);
        }

        console.log("files uploaded to sandbox")

        const command = sandbox.shells.run(``);
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
  async function fetchDirectory(owner: string, repo: string, path: string, branch: string) {
    const url = `https://api.github.com/repos/${owner}/${repo}/contents/${path}?ref=${branch}`;
    const response = await fetch(url);
    if (!response.ok) {
      throw new Error(`Failed to fetch ${url}: ${response.statusText}`);
    }
    const items = await response.json();
    let files = {};
    // Loop through all items in this directory
    for (const item of items) {
      if (item.type === 'file') {
        // Get file content from the download URL
        const fileResp = await fetch(item.download_url);
        if (!fileResp.ok) {
          throw new Error(`Failed to fetch file ${item.path}: ${fileResp.statusText}`);
        }
        const content = await fileResp.text();
        // Use the GitHub path as key; CodeSandbox expects an object with "content" property
        files[item.path] = { content };
      } else if (item.type === 'dir') {
        // Recursively fetch subdirectory
        const subFiles = await fetchDirectory(owner, repo, item.path, branch);
        files = { ...files, ...subFiles };
      }
    }
    return files;
  }

  async function uploadToCodeSandbox() {
    setLoading(true);
    setError(null);
    try {
      // Set repository details based on your URL:
      // https://github.com/dojimanetwork/linera-integration-demo/tree/improve-http-request-system-api/examples/universal-solver
      const owner = 'dojimanetwork';
      const repo = 'linera-integration-demo';
      const folderPath = 'examples/universal-solver';
      const branch = 'improve-http-request-system-api';

      // Recursively fetch files from the given folder
      const files = await fetchDirectory(owner, repo, folderPath, branch);
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
      <button onClick={ async () => await fetchGithubFiles(githubUrl)}>Submit</button>
      <button onClick={uploadToCodeSandbox}>Start CodeSandbox</button>
      
    </>
  );
}
