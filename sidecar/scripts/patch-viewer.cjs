#!/usr/bin/env node

// Patches prismarine-viewer to support MC versions not in its pre-built assets.
// Primary path: extracts textures, blockstates, and models directly from the
// Minecraft client JAR. Falls back to the minecraft-assets npm package when
// no JAR is available (CI, Docker, etc.).
//
// Set SKIP_VIEWER_PATCH=1 to skip entirely.
// Set MINECRAFT_JAR=/path/to/client.jar to override JAR discovery.

const path = require("path")
const fs = require("fs")
const os = require("os")
const { execSync } = require("child_process")

const TARGET_VERSION = process.env.MINECRAFT_VERSION || "1.21.9"
const VIEWER_ROOT = path.dirname(require.resolve("prismarine-viewer/package.json"))
const VERSION_FILE = path.join(VIEWER_ROOT, "viewer/lib/version.js")
const PUBLIC_DIR = path.join(VIEWER_ROOT, "public")

// ---------------------------------------------------------------------------
// JAR discovery
// ---------------------------------------------------------------------------

function findJar(version) {
  const candidates = []

  if (process.env.MINECRAFT_JAR) {
    candidates.push(process.env.MINECRAFT_JAR)
  }

  const home = os.homedir()

  // Flatpak (Linux)
  candidates.push(
    path.join(home, ".var/app/com.mojang.Minecraft/.minecraft/versions", version, `${version}.jar`),
  )
  // Native Linux
  candidates.push(path.join(home, ".minecraft/versions", version, `${version}.jar`))
  // macOS
  candidates.push(
    path.join(home, "Library/Application Support/minecraft/versions", version, `${version}.jar`),
  )
  // Windows
  if (process.env.APPDATA) {
    candidates.push(
      path.join(process.env.APPDATA, ".minecraft/versions", version, `${version}.jar`),
    )
  }

  for (const p of candidates) {
    if (fs.existsSync(p)) return p
  }

  throw new Error(
    `Could not find Minecraft ${version} client JAR. Searched:\n` +
      candidates.map(p => `  - ${p}`).join("\n"),
  )
}

// ---------------------------------------------------------------------------
// JAR extraction
// ---------------------------------------------------------------------------

function extractAssetsFromJar(jarPath) {
  const tmpdir = fs.mkdtempSync(path.join(os.tmpdir(), "golem-mc-assets-"))

  const cleanup = () => {
    try {
      fs.rmSync(tmpdir, { recursive: true, force: true })
    } catch {}
  }
  process.on("exit", cleanup)
  process.on("SIGINT", () => {
    cleanup()
    process.exit(1)
  })

  execSync(
    `unzip -o -q "${jarPath}" ` +
      `"assets/minecraft/blockstates/*" ` +
      `"assets/minecraft/models/block/*" ` +
      `"assets/minecraft/textures/block/*.png" ` +
      `-d "${tmpdir}"`,
    { stdio: "pipe" },
  )

  return tmpdir
}

// ---------------------------------------------------------------------------
// Build mcAssets-compatible object from extracted JAR contents
// ---------------------------------------------------------------------------

function buildMcAssets(tmpdir) {
  const blockstatesDir = path.join(tmpdir, "assets/minecraft/blockstates")
  const modelsDir = path.join(tmpdir, "assets/minecraft/models/block")
  const texturesDir = path.join(tmpdir, "assets/minecraft/textures/block")

  const blocksStates = {}
  for (const file of fs.readdirSync(blockstatesDir)) {
    if (!file.endsWith(".json")) continue
    blocksStates[file.replace(".json", "")] = JSON.parse(
      fs.readFileSync(path.join(blockstatesDir, file), "utf8"),
    )
  }

  const blocksModels = {}
  for (const file of fs.readdirSync(modelsDir)) {
    if (!file.endsWith(".json")) continue
    blocksModels[file.replace(".json", "")] = JSON.parse(
      fs.readFileSync(path.join(modelsDir, file), "utf8"),
    )
  }

  // atlas.js reads textures from mcAssets.directory + '/blocks' (plural)
  const stagingDir = path.join(tmpdir, "staging")
  const blocksDir = path.join(stagingDir, "blocks")
  fs.mkdirSync(blocksDir, { recursive: true })
  for (const file of fs.readdirSync(texturesDir)) {
    if (!file.endsWith(".png")) continue
    fs.copyFileSync(path.join(texturesDir, file), path.join(blocksDir, file))
  }

  return { blocksStates, blocksModels, directory: stagingDir }
}

// ---------------------------------------------------------------------------
// Shared: version patching & webpack rebuild
// ---------------------------------------------------------------------------

function isAlreadySupported() {
  const versionSrc = fs.readFileSync(VERSION_FILE, "utf8")
  return versionSrc.includes(`'${TARGET_VERSION}'`)
}

function patchSupportedVersions() {
  let src = fs.readFileSync(VERSION_FILE, "utf8")
  src = src.replace(/const supportedVersions = \[([^\]]+)\]/, (match, inner) => {
    if (inner.includes(`'${TARGET_VERSION}'`)) return match
    return `const supportedVersions = [${inner}, '${TARGET_VERSION}']`
  })
  fs.writeFileSync(VERSION_FILE, src)
  console.log(`[patch-viewer] Added '${TARGET_VERSION}' to supportedVersions`)
}

function rebuildBundles() {
  console.log("[patch-viewer] Rebuilding webpack bundles...")
  execSync("npx webpack --config " + path.join(VIEWER_ROOT, "webpack.config.js"), {
    cwd: VIEWER_ROOT,
    stdio: "inherit",
    env: { ...process.env, NODE_ENV: "production" },
  })
  console.log("[patch-viewer] Webpack rebuild complete")
}

// ---------------------------------------------------------------------------
// Asset generation pipeline (shared by both JAR and fallback paths)
// ---------------------------------------------------------------------------

function generateAssets(mcAssets) {
  const { makeTextureAtlas } = require(path.join(VIEWER_ROOT, "viewer/lib/atlas"))
  const { prepareBlocksStates } = require(path.join(VIEWER_ROOT, "viewer/lib/modelsBuilder"))
  const fsExtra = require("fs-extra")

  const texturesPath = path.join(PUBLIC_DIR, "textures")
  const blockStatesPath = path.join(PUBLIC_DIR, "blocksStates")

  const atlas = makeTextureAtlas(mcAssets)

  const pngFile = path.join(texturesPath, TARGET_VERSION + ".png")
  const stream = atlas.canvas.pngStream()
  const out = fs.createWriteStream(pngFile)

  return new Promise((resolve, reject) => {
    stream.on("data", chunk => out.write(chunk))
    stream.on("end", () => {
      console.log(`[patch-viewer] Generated textures/${TARGET_VERSION}.png`)

      const states = JSON.stringify(prepareBlocksStates(mcAssets, atlas))
      fs.writeFileSync(path.join(blockStatesPath, TARGET_VERSION + ".json"), states)
      console.log(`[patch-viewer] Generated blocksStates/${TARGET_VERSION}.json`)

      fsExtra.copySync(mcAssets.directory, path.join(texturesPath, TARGET_VERSION), {
        overwrite: true,
      })
      console.log(`[patch-viewer] Copied texture directory`)

      resolve()
    })
    stream.on("error", reject)
  })
}

// ---------------------------------------------------------------------------
// Primary path: extract from client JAR
// ---------------------------------------------------------------------------

async function generateFromJar() {
  const jarPath = findJar(TARGET_VERSION)
  console.log(`[patch-viewer] Extracting assets from JAR: ${jarPath}`)

  const tmpdir = extractAssetsFromJar(jarPath)
  const mcAssets = buildMcAssets(tmpdir)

  const bsCount = Object.keys(mcAssets.blocksStates).length
  const bmCount = Object.keys(mcAssets.blocksModels).length
  console.log(`[patch-viewer] Found ${bsCount} blockstates, ${bmCount} models`)

  await generateAssets(mcAssets)
}

// ---------------------------------------------------------------------------
// Fallback: minecraft-assets npm package
// ---------------------------------------------------------------------------

async function generateFromNpm() {
  console.log(`[patch-viewer] Falling back to minecraft-assets npm package`)
  const mcAssetsModule = require("minecraft-assets")
  const mcAssets = mcAssetsModule(TARGET_VERSION)
  console.log(
    `[patch-viewer] Using ${path.basename(mcAssets.directory)} textures (may be older version)`,
  )
  await generateAssets(mcAssets)
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

async function main() {
  if (process.env.SKIP_VIEWER_PATCH === "1") {
    console.log("[patch-viewer] SKIP_VIEWER_PATCH=1, skipping")
    return
  }

  if (isAlreadySupported()) {
    const atlasExists = fs.existsSync(path.join(PUBLIC_DIR, "textures", TARGET_VERSION + ".png"))
    if (atlasExists) {
      console.log(`[patch-viewer] prismarine-viewer already supports ${TARGET_VERSION}, skipping`)
      return
    }
  }

  console.log(`[patch-viewer] Patching prismarine-viewer for MC ${TARGET_VERSION}`)
  patchSupportedVersions()

  try {
    await generateFromJar()
  } catch (jarErr) {
    console.warn(`[patch-viewer] JAR extraction failed: ${jarErr.message}`)
    try {
      await generateFromNpm()
    } catch (npmErr) {
      throw new Error(
        `Both asset sources failed.\n` + `  JAR: ${jarErr.message}\n` + `  npm: ${npmErr.message}`,
      )
    }
  }

  rebuildBundles()
  console.log(`[patch-viewer] Done! Viewer now supports ${TARGET_VERSION}`)
}

main().catch(err => {
  console.error("[patch-viewer] Fatal:", err)
  process.exit(1)
})
