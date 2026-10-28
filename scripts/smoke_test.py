import json
import subprocess
import sys
import time

def main():
    print("[1/9] Spawning ./bin/isa-afp-mcp process over stdio...")
    proc = subprocess.Popen(
        ["./bin/isa-afp-mcp", "--log-level=info"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        bufsize=1
    )

    msg_id = 0
    def send_rpc(method, params=None):
        nonlocal msg_id
        msg_id += 1
        req = {
            "jsonrpc": "2.0",
            "id": msg_id,
            "method": method,
        }
        if params is not None:
            req["params"] = params
        
        raw = json.dumps(req)
        proc.stdin.write(raw + "\n")
        proc.stdin.flush()
        line = proc.stdout.readline()
        if not line:
            stderr = proc.stderr.read()
            raise RuntimeError(f"Server closed connection unexpectedly. Stderr: {stderr}")
        return json.loads(line)

    try:
        # 1. Initialize
        print("[2/9] Sending initialize request...")
        init_res = send_rpc("initialize", {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "smoke-test-client", "version": "1.0.0"}
        })
        print(f" -> Initialized: Server Name={init_res['result']['serverInfo']['name']}, Version={init_res['result']['serverInfo']['version']}")

        # Notification: initialized
        proc.stdin.write(json.dumps({"jsonrpc": "2.0", "method": "notifications/initialized"}) + "\n")
        proc.stdin.flush()

        # 2. List tools
        print("[3/9] Listing registered tools...")
        tools_res = send_rpc("tools/list")
        tools = tools_res["result"]["tools"]
        tool_names = [t["name"] for t in tools]
        print(f" -> Registered {len(tools)} tools: {', '.join(tool_names)}")
        assert "search_entries" in tool_names
        assert "get_entry" in tool_names
        assert "get_theory_outline" in tool_names
        assert "get_root" in tool_names

        # 3. Check catalog status
        print("[4/9] Checking catalog status...")
        status_res = send_rpc("tools/call", {
            "name": "get_catalog_status",
            "arguments": {}
        })
        print(f" -> Status result: {status_res['result']['content'][0]['text']}")

        # 4. Search entries
        print("[5/9] Testing search_entries for 'Dijkstra'...")
        t0 = time.perf_counter()
        search_res = send_rpc("tools/call", {
            "name": "search_entries",
            "arguments": {"query": "Dijkstra", "limit": 3}
        })
        t1 = time.perf_counter()
        search_text = search_res["result"]["content"][0]["text"]
        print(f" -> Search took {(t1 - t0)*1000:.2f}ms roundtrip.")
        print(f" -> Result snippet:\n{search_text[:300]}...\n")

        # 5. Get entry
        print("[6/9] Testing get_entry for 'Formal_SSA'...")
        entry_res = send_rpc("tools/call", {
            "name": "get_entry",
            "arguments": {"shortname": "Formal_SSA"}
        })
        entry_text = entry_res["result"]["content"][0]["text"]
        print(f" -> Entry header:\n{entry_text[:250]}...\n")

        # 6. Get ROOT session dependencies
        print("[7/9] Testing get_root for 'Formal_SSA'...")
        root_res = send_rpc("tools/call", {
            "name": "get_root",
            "arguments": {"shortname": "Formal_SSA"}
        })
        root_text = root_res["result"]["content"][0]["text"]
        print(f" -> ROOT details:\n{root_text[:250]}...\n")

        # 7. Get theory outline
        print("[8/9] Testing get_theory_outline for 'Formal_SSA / Construct_SSA'...")
        outline_res = send_rpc("tools/call", {
            "name": "get_theory_outline",
            "arguments": {
                "shortname": "Formal_SSA",
                "theory": "Construct_SSA"
            }
        })
        outline_text = outline_res["result"]["content"][0]["text"]
        print(f" -> Outline snippet:\n{outline_text[:350]}...\n")

        # 8. Suggest keywords
        print("[9/9] Testing suggest_keywords for 'elliptic'...")
        kw_res = send_rpc("tools/call", {
            "name": "suggest_keywords",
            "arguments": {"prefix": "elliptic", "limit": 5}
        })
        kw_text = kw_res["result"]["content"][0]["text"]
        print(f" -> Keywords:\n{kw_text}\n")

        print("=== ALL 9 SMOKE TEST PHASES PASSED SUCCESSFULLY ===")

    finally:
        proc.terminate()
        proc.wait(timeout=5)

if __name__ == "__main__":
    main()
