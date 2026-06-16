# qr-multi-imgs

**Scan a folder → decode QR → organize, export, recreate — da una TUI pulita.**

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Go Report](https://goreportcard.com/badge/github.com/thousandflowers/qr-multi-imgs)](https://goreportcard.com/report/github.com/thousandflowers/qr-multi-imgs)
[![CI](https://github.com/thousandflowers/qr-multi-imgs/actions/workflows/ci.yml/badge.svg)](https://github.com/thousandflowers/qr-multi-imgs/actions/workflows/ci.yml)
[![Zero Deps](https://img.shields.io/badge/deps-0-darkgreen)]()
[![Speed](https://img.shields.io/badge/scan-1300%20img%2Fs-purple)]()

```bash
bash <(curl -sL https://raw.githubusercontent.com/thousandflowers/qr-multi-imgs/main/install.sh)
qr-multi-imgs ~/Desktop/qr-test
```

**~1300 immagini/secondo** su M2 Pro. Single binary Go. Zero dipendenze di sistema.

---

## Perché

Altri scanner CLI esistono, ma nessuno ha una TUI completa per batch:

| | qr-multi-imgs | qr-scanner-cli | qrtool |
|---|---|---|---|
| TUI interattiva | ✅ | ❌ | ❌ |
| Organizza file (with/without QR) | ✅ | ❌ | ❌ |
| Elimina senza QR | ✅ | ❌ | ❌ |
| Ricrea QR da contenuto | ✅ | ❌ | ✅ |
| Esporta (JSON/CSV/TXT) | ✅ | ❌ | ✅ |
| Drag & drop + clipboard | ✅ | ❌ | ❌ |
| Pure Go, zero CGo | ✅ | ❌ (npm) | ✅ |

---

## Azioni TUI

| Tasto | Azione |
|:-----:|--------|
| `l` | Elenca risultati |
| `e` | Esporta (JSON/CSV/TXT) |
| `d` | Elimina immagini senza QR |
| `o` | Organizza in `with_qr/` e `without_qr/` |
| `r` | Ricrea QR 512×512 dai contenuti |
| `q` / `^C` | Esci |

### 5 modi di input

Argomento CLI, scrivi path, drag & drop, incolla (`Cmd+V`), clipboard (`Ctrl+D`).

---

## Install

```bash
# One-liner
bash <(curl -sL https://raw.githubusercontent.com/thousandflowers/qr-multi-imgs/main/install.sh)

# Da source
git clone https://github.com/thousandflowers/qr-multi-imgs.git
cd qr-multi-imgs && go build
```

**Requisiti:** Go 1.26+, terminale ANSI. Supporta PNG/JPEG/GIF/BMP/WebP.

---

## License

MIT — [LICENSE](LICENSE).
