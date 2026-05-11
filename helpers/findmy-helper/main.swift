import AppKit
import CoreGraphics
import Foundation
import ScreenCaptureKit
import Vision

func die(_ msg: String, code: Int32 = 1) -> Never {
    FileHandle.standardError.write(Data((msg + "\n").utf8))
    exit(code)
}

func emit<T: Encodable>(_ value: T) {
    let enc = JSONEncoder()
    enc.outputFormatting = [.sortedKeys]
    guard let data = try? enc.encode(value) else { die("encode failed") }
    FileHandle.standardOutput.write(data)
    FileHandle.standardOutput.write(Data([0x0a]))
}

struct WindowInfo: Encodable {
    let pid: Int
    let windowID: Int
    let layer: Int
    let title: String
    let x: Int
    let y: Int
    let width: Int
    let height: Int
    let onScreen: Bool
}

func cmdWindow(_ args: [String]) {
    var owner: String?
    var i = 0
    while i < args.count {
        switch args[i] {
        case "--owner":
            i += 1
            owner = i < args.count ? args[i] : nil
        default: break
        }
        i += 1
    }
    guard let owner else { die("usage: findmy-helper window --owner <name>") }
    guard let arr = CGWindowListCopyWindowInfo([.optionAll], kCGNullWindowID) as? [[String: Any]] else { die("CGWindowListCopyWindowInfo failed") }
    var out: [WindowInfo] = []
    for w in arr {
        guard let name = w[kCGWindowOwnerName as String] as? String, name == owner else { continue }
        let layer = (w[kCGWindowLayer as String] as? Int) ?? 0
        let onScreen = (w[kCGWindowIsOnscreen as String] as? Bool) ?? false
        guard let bounds = w[kCGWindowBounds as String] as? [String: Any],
              let h = bounds["Height"] as? Int, let wd = bounds["Width"] as? Int,
              let x = bounds["X"] as? Int, let y = bounds["Y"] as? Int else { continue }
        out.append(WindowInfo(
            pid: (w[kCGWindowOwnerPID as String] as? Int) ?? 0,
            windowID: (w[kCGWindowNumber as String] as? Int) ?? 0,
            layer: layer,
            title: (w[kCGWindowName as String] as? String) ?? "",
            x: x, y: y, width: wd, height: h,
            onScreen: onScreen
        ))
    }
    emit(out)
}

struct TextLine: Encodable {
    let text: String
    let confidence: Double
    let x: Int
    let y: Int
    let width: Int
    let height: Int
}

func cmdOCR(_ args: [String]) {
    guard let path = args.first else { die("usage: findmy-helper ocr <image>") }
    let url = URL(fileURLWithPath: path)
    guard let img = NSImage(contentsOf: url),
          let cg = img.cgImage(forProposedRect: nil, context: nil, hints: nil) else { die("cannot load image: \(path)") }
    let req = VNRecognizeTextRequest()
    req.recognitionLevel = .accurate
    // Language correction mangles proper nouns ("Shahine" → "Sunshine"); names
    // come through cleaner with it off.
    req.usesLanguageCorrection = false
    let handler = VNImageRequestHandler(cgImage: cg)
    do { try handler.perform([req]) } catch { die("vision failed: \(error)") }
    let h = Double(cg.height), w = Double(cg.width)
    var out: [TextLine] = []
    for obs in (req.results ?? []) {
        guard let cand = obs.topCandidates(1).first else { continue }
        let bb = obs.boundingBox
        out.append(TextLine(
            text: cand.string,
            confidence: Double(cand.confidence),
            x: Int(bb.minX * w),
            y: Int((1.0 - bb.maxY) * h),
            width: Int(bb.width * w),
            height: Int(bb.height * h)
        ))
    }
    emit(out)
}

func cmdClick(_ args: [String]) {
    guard args.count >= 2, let x = Double(args[0]), let y = Double(args[1]) else {
        die("usage: findmy-helper click <x> <y>")
    }
    let pt = CGPoint(x: x, y: y)
    let src = CGEventSource(stateID: .hidSystemState)
    let down = CGEvent(mouseEventSource: src, mouseType: .leftMouseDown, mouseCursorPosition: pt, mouseButton: .left)
    let up = CGEvent(mouseEventSource: src, mouseType: .leftMouseUp, mouseCursorPosition: pt, mouseButton: .left)
    down?.post(tap: .cghidEventTap)
    usleep(40_000)
    up?.post(tap: .cghidEventTap)
    print("{\"ok\":true}")
}

struct Permissions: Encodable {
    let screenRecording: Bool
    let accessibility: Bool
}

// cmdPermissions reports whether this process holds the TCC grants needed to
// capture FindMy.app and synthesize clicks. CGPreflightScreenCaptureAccess()
// is unreliable for CLI binaries (TCC entries can be stale across rebuilds),
// so when it reports false we exercise the permission via SCShareableContent —
// the only definitive probe.
func cmdPermissions(_ args: [String]) {
    var screenRecording = CGPreflightScreenCaptureAccess()
    if !screenRecording {
        let sem = DispatchSemaphore(value: 0)
        SCShareableContent.getWithCompletionHandler { content, err in
            screenRecording = (content != nil && err == nil)
            sem.signal()
        }
        _ = sem.wait(timeout: .now() + 3.0)
    }
    let accessibility: Bool
    if #available(macOS 14.0, *) {
        accessibility = CGPreflightPostEventAccess()
    } else {
        accessibility = AXIsProcessTrusted()
    }
    emit(Permissions(screenRecording: screenRecording, accessibility: accessibility))
}

let args = Array(CommandLine.arguments.dropFirst())
guard let sub = args.first else {
    die("usage: findmy-helper {window|ocr|click|permissions} ...")
}
let rest = Array(args.dropFirst())
switch sub {
case "window": cmdWindow(rest)
case "ocr": cmdOCR(rest)
case "click": cmdClick(rest)
case "permissions": cmdPermissions(rest)
default: die("unknown subcommand: \(sub)")
}
