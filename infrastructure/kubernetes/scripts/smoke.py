"""Disposable K3s schema/admission smoke test; no import or training.

Requires Docker, kubectl and scripts/requirements.txt. Only creates/removes the
fixed test container real-estate-readiness-k3s. Never uses the operator's context.
Use --existing to reuse that explicitly named test container.
"""

import os
import subprocess
import sys
import tempfile
import time
import urllib.request
from pathlib import Path

import yaml

from validate import OVERLAYS, ROOT

CONTAINER = "real-estate-readiness-k3s"


def run(args, **kwargs):
    return subprocess.check_output(args, text=True, **kwargs)


def main():
    created = "--existing" not in sys.argv
    if created:
        run(["docker", "run", "-d", "--privileged", "--name", CONTAINER,
             "-p", "127.0.0.1:6445:6443", "rancher/k3s:v1.35.8-k3s1",
             "server", "--cluster-init", "--disable", "traefik",
             "--disable", "servicelb", "--disable", "metrics-server"])
    try:
        with tempfile.TemporaryDirectory(prefix="real-estate-k3s-") as directory:
            config = Path(directory) / "kubeconfig"
            for attempt in range(60):
                try:
                    content = run(["docker", "exec", CONTAINER, "cat", "/etc/rancher/k3s/k3s.yaml"])
                    config.write_text(content.replace("127.0.0.1:6443", "127.0.0.1:6445"))
                    config.chmod(0o600)
                    break
                except subprocess.CalledProcessError:
                    time.sleep(2)
            else:
                raise RuntimeError("K3s did not create kubeconfig")
            env = {**os.environ, "KUBECONFIG": str(config)}
            kube = ["kubectl", "--context", "default"]
            for attempt in range(60):
                try:
                    run(kube + ["get", "--raw=/readyz"], env=env)
                    break
                except subprocess.CalledProcessError:
                    time.sleep(2)
            else:
                raise RuntimeError("K3s API not ready")
            print(run(kube + ["get", "nodes"], env=env))
            urls = {}
            for line in (ROOT / "addons/versions.env").read_text().splitlines():
                key, value = line.split("=", 1)
                if key.endswith("_MANIFEST"):
                    urls[key] = value
            # CRD validation only; no pretend backup/failover test without operators.
            for url in urls.values():
                with urllib.request.urlopen(url, timeout=60) as response:
                    documents = list(yaml.safe_load_all(response.read()))
                crds = [d for d in documents if d and d.get("kind") == "CustomResourceDefinition"]
                print(run(kube + ["apply", "--server-side", "-f", "-"],
                          input=yaml.safe_dump_all(crds), env=env))
            run(kube + ["wait", "--for=condition=Established", "crd", "--all", "--timeout=120s"], env=env)
            run(kube + ["apply", "-f", str(ROOT / "security/namespace.yaml")], env=env)
            for overlay in OVERLAYS:
                rendered = run(["kubectl", "kustomize", str(ROOT / overlay)])
                for mode in ("client", "server"):
                    result = run(kube + ["apply", "--dry-run=" + mode, "-f", "-"],
                                 input=rendered, env=env)
                    print(f"PASS {mode} {overlay}\n{result}")
            phase_env = {**env, "KUBE_CONTEXT": "default",
                         "IMAGE_TAG": "sha-" + "0" * 40,
                         "PUBLIC_HOST": "readiness.test",
                         "ACME_EMAIL": "operator@readiness.test"}
            print(run(["sh", str(ROOT / "scripts/operator.sh"),
                       "apply", "single-node", "configuration"], env=phase_env))
            deployments = yaml.safe_load(run(
                kube + ["-n", "real-estate", "get", "deployments", "-o", "yaml"], env=env))
            assert deployments["items"] == [], "Configuration phase must not launch applications"
            run(kube + ["-n", "real-estate", "get", "pvc", "pipeline-data"], env=env)
            print("PASS configuration phase creates prerequisites, not application Deployments")
    finally:
        if created:
            # Exact disposable container created by this invocation, never user data.
            run(["docker", "rm", "-f", "-v", CONTAINER])


if __name__ == "__main__":
    main()
