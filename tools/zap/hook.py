# ZAP baseline hook. Requests the fixed URL list so the passive scanner sees
# the pages and API routes the spider from / may not reach. Only URLs on the
# local target are ever requested.
import os


def zap_started(zap, target):
    base = target.rstrip("/")
    for path in os.environ.get("ZAP_PATHS", "").split():
        zap.core.access_url(base + path, followredirects="true")
