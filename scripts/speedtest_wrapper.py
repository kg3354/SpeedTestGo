import requests
import hashlib
import sys

class SpeedTestClient:
    def __init__(self, base_url):
        self.base_url = base_url

    def init_download(self, size_mb):
        """Initialize a download session."""
        url = f"{self.base_url}/download/init"
        data = {"size_mb": size_mb}
        response = requests.post(url, json=data)
        response.raise_for_status()
        return response.json()

    def download_file(self, session_id, output_path):
        """Download the file for the given session ID."""
        url = f"{self.base_url}/download/data?session_id={session_id}"
        with requests.get(url, stream=True) as response:
            response.raise_for_status()
            with open(output_path, "wb") as f:
                for chunk in response.iter_content(chunk_size=8192):
                    f.write(chunk)
        print(f"File downloaded successfully: {output_path}")

    def compute_hash(self, file_path):
        """Compute the SHA-256 hash of a file."""
        sha256 = hashlib.sha256()
        with open(file_path, "rb") as f:
            while chunk := f.read(8192):
                sha256.update(chunk)
        return sha256.hexdigest()

    def verify_download(self, session_id, computed_hash):
        """Verify the downloaded file's hash with the server."""
        url = f"{self.base_url}/download/verify"
        data = {"session_id": session_id, "computed_hash": computed_hash}
        response = requests.post(url, json=data)
        response.raise_for_status()
        return response.json()

    def get_speed(self, session_id):
        """Fetch the download speed."""
        url = f"{self.base_url}/download/speed?session_id={session_id}"
        response = requests.get(url)
        response.raise_for_status()
        return response.json()

if __name__ == "__main__":
    # Base URL of the speed test server
    BASE_URL = "http://localhost:8080"
    client = SpeedTestClient(BASE_URL)

    try:
        # Step 1: Initialize the download
        size_mb = 1000  # Change file size as needed
        session_info = client.init_download(size_mb)
        session_id = session_info["session_id"]
        expected_hash = session_info["expected_hash"]
        print(f"Session initialized: {session_id}")

        # Step 2: Download the file
        output_file = "downloaded.bin"
        client.download_file(session_id, output_file)

        # Step 3: Fetch download speed
        speed_info = client.get_speed(session_id)
        print(f"Download Speed: {speed_info['download_speed_mbps']} Mbps")

        # Step 4: Compute the hash of the downloaded file
        computed_hash = client.compute_hash(output_file)
        print(f"Computed Hash: {computed_hash}")

        # Step 5: Verify the hash with the server
        if computed_hash == expected_hash:
            verification_response = client.verify_download(session_id, computed_hash)
            print(f"Verification Status: {verification_response['status']}")
        else:
            print("Hash mismatch! Aborting speed verification.")
            sys.exit(1)

    except requests.RequestException as e:
        print(f"Error: {e}", file=sys.stderr)
    except Exception as e:
        print(f"Unexpected Error: {e}", file=sys.stderr)
