import type { NextConfig } from 'next'

//! output standalone is what makes the docker runtime stage small, it emits a tree with only
//! the files the server actually needs instead of the whole node_modules
//? outputFileTracingRoot points at the repo root because @shared/* resolves to src/shared,
//? a sibling of src/web. without it next traces from src/web and leaves the shared files out
const config: NextConfig = {
  reactStrictMode: true,
  output: 'standalone',
  outputFileTracingRoot: __dirname + '/../..',
}

export default config
