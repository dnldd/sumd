# Data Integrity Verification

On May 6th 2017,  it was discovered the popular video transcoder [Handbrake](https://handbrake.fr/) had been hacked. A compromised download mirror was used to serve a modified version of the app (v1.0.7) which infected updating users with the proton trojan. Before the hack was discovered the malware had already been used to steal credentials of a lead developer at [Panic Inc](https://panic.com/). [Source code and proprietary information were stolen](https://panic.com/blog/stolen-source-code/) as a result.

## The Status Quo
Current software distribution processes rely solely on the end user's technical prowess to verify the authenticity of downloaded software. This is careless considering most users are not knowledgeable enough to perform data integrity verification by themselves, even with a detailed tutorial. The status quo exposes users to unnecessary risk since current software distribution process can be augmented with data integrity verification checks.

## Automating Data Integrity Checks
A Software distribution process with an automated data integrity check has the advantage of preventing the propagation of malicious software in real-time. This will be done by comparing checksums of requested software downloads against their corresponding public, unalterable and versioned release history.

Politeia being a system for storing off-chain data that is both versioned and timestamped facilitates integrating data integrity checks into software distribution process. Politeia will serve as the verifiable release history record store that download requests will be verified against.

The format for a release history entry is as follows:
- metadata:
  ```
  {
    "checksum":"hash", // the hash of the release
    "product": "software", // the name of the software
    "version": "version number", // the version number of the release
    "file": "filename", // the filename of the release
  }
  ```
- file: a markdown file of the release notes.


For this modified distribution process, download servers or mirrors become responsible for ensuring the authenticity of release software before fulfilling a user's download request. With only the release file metadata as input, the download servers must be able to:
  - accurately locate the release file referenced.
  - verify the authenticity of the file by asserting the distribution checksum is the same as the release file checksum.
  - generate a unique, time-bound download url for each download request.

`sumd` is a reference implementation of a download server with the capabilities listed above. The download server leverages a structured release directory to ensure it can always find the release file by forming a path to it using information in the metadata. The path constructed is of the form:
 ```
    /[releasedir]/[product]/[version]/[file]
 ```

The payload returned by the server is structured as follows:
  - on successful verification:
   ```
  {
    "releasechecksum": "hash",
    "distributionchecksum": "hash",
    "verified": true,
    "download": "url",
  }
  ```

  - on verification failure:
  ```
  {
    "releasechecksum": "hash",
    "distributionchecksum": "hash",
    "verified": false,
    "error": "details",
  }
  ```

## Further Improvements
The download server currently calculates checksums on demand. It would be more efficient to use a file system watcher to trigger checksum recalculations when a release file is either newly added or updated. This would speed up the verification process significantly because release checksums would be readily available for every incoming download request.

The download server also needs rate limit requests for download links and endpoints to mitigate DDOS attacks.
