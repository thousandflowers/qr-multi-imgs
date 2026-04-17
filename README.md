# QR Multi IMGs

> Rileva QR code da immagini anche difficili - sfocati, piccoli, ruotati, di bassa qualità

![Version](https://img.shields.io/badge/version-v0.6.0--Enhanced-blue)
![License](https://img.shields.io/badge/license-MIT-green)
![Python](https://img.shields.io/badge/python-3.12+-blue)

I QR code nelle foto reali sono spesso sfocati, tagliati male, molto piccoli o ruotati. QR Multi IMGs usa 11 metodi di rilevazione progressivi per trovare QR code che altri scanner perdono.

---

## Perché questo tool

I QR code nelle foto reali sono spesso:
- **Sfocati** o fuori fuoco
- **Tagliati** male
- **Piccoli** o molto grandi
- **Ruotati** ad angoli strani
- Di **bassa qualità**

QR Multi IMGs affronta questi problemi con una pipeline di 11 metodi che si attivano progressivamente.

---

## Installazione

### Homebrew (macOS consigliato)

```bash
brew tap thousandflowers/qr-multi-imgs
brew install qr-multi-imgs
qr-multi-imgs --path ./images --action list
```

### pip

```bash
pip install -r requirements.txt
python3 qr_multi_imgs.py --path ./images --action list
```

### Dipendenze

- Python 3.12+
- zbar (`brew install zbar`)

---

## Quick Start

### Interattivo

```bash
python3 qr_multi_imgs.py
```

### CLI Mode

```bash
# Scansione base
python3 qr_multi_imgs.py --path ./images --action list

# Con dettagli
python3 qr_multi_imgs.py --path ./images --verbose

# Massimo rilevazione (prova tutto)
python3 qr_multi_imgs.py --path ./images --force-deep --verbose

# Esporta risultati
python3 qr_multi_imgs.py --path ./images --action export --export-format json
```

---

## Features

| Feature | Descrizione |
|---------|-------------|
| **11 Detection Methods** | Pipeline progressiva per QR difficili |
| **10 Azioni** | list, export, delete, organize, recreate, extract, decode, filter, batch-rename, verify |
| **Memory Safe** | Chiusura corretta di tutte le immagini |
| **Thread-Safe** | Processing parallelo |
| **Path Validation** | protezione path traversal |
| **Deep Scan** | Rilevazione migliorata |
| **Verbose** | Log dettagliato per debug |

---

## Rilevazione QR Code

### Metodi Disponibili

| # | Metodo | Fase | Usi Per |
|---|--------|------|---------|
| 1 | Basic decode | 1 | QR normali |
| 2 | Grayscale | 1 | Basso contrasto |
| 3 | Contrast+Unsharp | 1 | preprocessing base |
| 4 | Sharpen | 2 | QR sfocati |
| 5 | Deblur | 2 | QR molto sfocati |
| 6 | Rotation | 3 | QR ruotati |
| 7 | Multi-scale | 3 | QR di dimensioni variabili |
| 8 | QReader | 3 | ML-based detection |
| 9 | Adaptive | 3 | Bassa qualità |
| 10 | Morphology | 3 | QR danneggiati |
| 11 | Extreme Scale | Full | QR molto piccoli/grandi |

### Detection Flow

```
Phase 1 (sempre)
├── Basic decode
├── Grayscale
├── Contrast + Unsharp
└── Resize 2x

Phase 2 (deep_scan)
├── Sharpen ( blur)
├── Deblur (blur estremo)
└── Resize 3x

Phase 3 (force_deep)
├── Rotation (90°/180°/270°)
├── Multi-scale (0.5x-3x)
├── QReader (ML)
├── Adaptive threshold
└── Morphology

Full (fallback)
├── Extreme scale (4x-8x)
└── Method 11
```

---

## Comandi CLI

### Azioni

| Azione | Descrizione | Esempio |
|--------|-------------|---------|
| `list` | Mostra risultati | `--action list` |
| `export` | Salva su file | `--action export --export-format json` |
| `delete` | Elimina senza QR | `--action delete --confirm` |
| `organize` | Sposta in cartelle | `--action organize --move` |
| `recreate` | Crea nuovi QR | `--action recreate --qr-format png` |
| `extract` | Estrai regioni QR | `--action extract --padding 20` |
| `decode` | Solo contenuto | `--action decode` |
| `filter` | Filtra per pattern | `--action filter --filter-pattern "mcdonalds"` |
| `batch-rename` | Rinomina batch | `--action batch-rename --confirm` |
| `verify` | Verifica QR | `--action verify --output ./recreated` |

### Opzioni

| Opzione | Alias | Default | Descrizione |
|--------|-------|---------|-------------|
| `--path` | `-p` | (richiesto) | Cartella immagini |
| `--action` | `-a` | `list` | Azione |
| `--recursive` | `-r` | false | Subfolders |
| `--verbose` | `-v` | false | Dettagliato |
| `--force-deep` | | false | Massimo rilevazione |
| `--deep-scan` | | true | Rilevazione avanzata |
| `--parallel` | | false | Processing parallelo |
| `--output` | `-o` | auto | Cartella output |
| `--confirm` | | false | Skip conferma |
| `--timeout` | `-t` | 15 | Timeout per immagine |

---

## Configurazione

### Timeout

- Default: 15 secondi per immagine
- Deep: 30 secondi
- Usa `--timeout 60` per immagini grandi

### Image Formats

jpg, jpeg, png, bmp, gif, webp, tiff, tif (case-insensitive)

---

## Troubleshooting

### "zbar library not found"

```bash
# macOS Apple Silicon
brew install zbar
export DYLD_LIBRARY_PATH=/opt/homebrew/lib

# macOS Intel
brew install zbar
export DYLD_LIBRARY_PATH=/usr/local/lib
```

### Scanning lento

```bash
# Usa parallel per molte immagini
python3 qr_multi_imgs.py --path ./images --parallel --progress

# Disabilita deep scan per scan base veloce
python3 qr_multi_imgs.py --path ./images --deep-scan=false
```

### Tante immagini non rilevate

```bash
# Prova maximum detection
python3 qr_multi_imgs.py --path ./images --force-deep --verbose
```

---

## License

MIT License - vedi file LICENSE

## Crediti

- **Author**: QR Multi IMGS Team
- **GitHub**: https://github.com/thousandflowers/qr-multi-imgs
- **Issues**: https://github.com/thousandflowers/qr-multi-imgs/issues