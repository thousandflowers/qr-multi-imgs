// Every language the page speaks, as data.
//
// English is the reference: a key missing from another language falls back to
// it rather than rendering the key itself, and the self-test in i18n.js fails
// if any language is short a key, so a string cannot be added in one language
// and forgotten in the rest.
//
// Adding a language means adding one block here and nothing else.

(function (global) {
  "use strict";

  const S = {};

  // Written in each language's own words, because that is how a person finds
  // their own language in a list.
  const NAMES = {
    en: "English", it: "Italiano", es: "Español", fr: "Français",
    de: "Deutsch", pt: "Português",
  };

  S.en = {
    "app.tagline": "QR scanner",
    "nav.source": "Source",

    "hero.title": "Read, filter and export every QR code in a folder of images",
    "hero.sub": "Most scanners take one picture at a time. Drop a whole folder — or one photo, your camera, or whatever is on the clipboard — and get every code in every image at once.",
    "hero.trust": "Nothing is uploaded — it all runs in this tab",

    "tab.images": "Images",
    "tab.camera": "Camera",

    "drop.title": "Drag & drop, or browse",
    "drop.sub": "One photo, a folder, several folders, or a zip",
    "drop.browse": "Browse files",
    "drop.folder": "Choose folder",
    "drop.paste": "Paste an image",
    "drop.shoot": "Take a photo",

    "cam.off": "The camera is off.<br>Point it at a code and it reads it live.",
    "cam.start": "Start camera",
    "cam.stop": "Stop camera",
    "cam.looking": "Looking for a code…",
    "cam.denied": "Camera permission was declined.",
    "cam.none": "No camera available here.",
    "cam.read": "Read {n} code(s) so far.",

    "res.title": "Results",
    "res.private": "100% private · runs in your browser",
    "res.empty": "Codes you scan will show up here.",
    "res.nothing": "Nothing there this page can read — pictures only.",
    "res.search": "Search inside the codes…",
    "res.noMatch": "No result matches that search.",
    "res.noCode": "No code in this image",
    "engine.loading": "starting the reader",
    "engine.ready": "reader ready",

    "stat.images": "images",
    "stat.with": "with a code",
    "stat.codes": "codes",
    "stat.unique": "unique",
    "stat.without": "without",
    "stat.unreadable": "unreadable",

    "act.copyAll": "Copy all",
    "act.copy": "copy",
    "act.copied": "copied",
    "act.download": "Download",
    "act.clear": "Clear",
    "act.cancel": "Cancel",
    "act.clearAsk": "Throw away every result read so far?",
    "act.open": "Open",
    "act.saveContact": "Save contact",
    "act.reveal": "Reveal",
    "act.hide": "Hide",

    "rename.pattern": "New name",
    "rename.code": "The code content",
    "rename.short": "The meaningful part of the code",
    "rename.nameCode": "Original name + code",
    "rename.keep": "Keep the original names",
    "rename.zip": "Download ZIP ({n})",
    "rename.none": "No image here has a code to be named after.",
    "rename.preview": "will be saved as",

    "dupe.badge": "in {n} images",

    "err.advice": "Retake the photo closer, with the code flat and evenly lit — or crop tight around it and try again.",

    "adv.title": "Prefer the terminal? Use it there instead",
    "adv.sub": "The same reader as a command you can put in a script — and, with it running, this page can also organize and rename the files where they sit. Nothing here is needed to read a code.",
    "adv.localOn": "A local copy of the program is answering, so this page can also act on files on disk.",
    "adv.scan": "Scan on disk",
    "adv.paths": "Folders or files, one per line",
    "adv.recreate": "Recreate QR images",
    "adv.organize": "Sort into with_qr / without_qr",
    "adv.delete": "Delete images without a code",
    "adv.confirmOrganize": "Move every scanned image into a with_qr / without_qr folder beside it?",
    "adv.confirmDelete": "Permanently delete every scanned image that has no QR code. This cannot be undone. Continue?",
    "adv.scanFirst": "Scan on disk first.",

    "foot.privacy": "Your images never leave this device.",

    "kind.url": "Link", "kind.wifi": "Wi-Fi network", "kind.contact": "Contact",
    "kind.payment": "Payment", "kind.email": "Email", "kind.phone": "Phone number",
    "kind.sms": "Text message", "kind.location": "Location", "kind.event": "Event",
    "kind.crypto": "Crypto address", "kind.otp": "Two-factor setup",
    "kind.reference": "Reference", "kind.text": "Text",

    "f.site": "Site", "f.reference": "Reference", "f.address": "Address",
    "f.network": "Network", "f.security": "Security", "f.password": "Password",
    "f.name": "Name", "f.phone": "Phone", "f.email": "Email", "f.company": "Company",
    "f.payee": "Payee", "f.iban": "IBAN", "f.amount": "Amount",
    "f.subject": "Subject", "f.message": "Message", "f.number": "Number",
    "f.latitude": "Latitude", "f.longitude": "Longitude",
    "f.title": "Title", "f.starts": "Starts", "f.ends": "Ends", "f.where": "Where",
    "f.account": "Account", "f.issuer": "Issuer", "f.algorithm": "Algorithm",
    "f.secret": "Secret key", "f.content": "Content",
  };

  S.it = {
    "app.tagline": "lettore QR",
    "nav.source": "Codice",

    "hero.title": "Leggi, filtra ed esporta ogni codice QR in una cartella di immagini",
    "hero.sub": "Quasi tutti i lettori prendono una foto alla volta. Qui trascini una cartella intera — o una sola foto, la fotocamera, o quello che hai negli appunti — e ottieni ogni codice di ogni immagine in un colpo solo.",
    "hero.trust": "Non viene caricato niente — funziona tutto in questa scheda",

    "tab.images": "Immagini",
    "tab.camera": "Fotocamera",

    "drop.title": "Trascina qui, oppure scegli",
    "drop.sub": "Una foto, una cartella, più cartelle, o uno zip",
    "drop.browse": "Scegli i file",
    "drop.folder": "Scegli una cartella",
    "drop.paste": "Incolla un'immagine",
    "drop.shoot": "Scatta una foto",

    "cam.off": "La fotocamera è spenta.<br>Inquadra un codice e lo legge dal vivo.",
    "cam.start": "Accendi la fotocamera",
    "cam.stop": "Spegni la fotocamera",
    "cam.looking": "Sto cercando un codice…",
    "cam.denied": "Permesso della fotocamera negato.",
    "cam.none": "Qui non c'è una fotocamera disponibile.",
    "cam.read": "Letti {n} codici finora.",

    "res.title": "Risultati",
    "res.private": "100% privato · funziona nel tuo browser",
    "res.empty": "I codici che leggi compaiono qui.",
    "res.nothing": "Lì dentro non c'è niente che questa pagina sappia leggere — solo immagini.",
    "res.search": "Cerca dentro i codici…",
    "res.noMatch": "Nessun risultato corrisponde alla ricerca.",
    "res.noCode": "Nessun codice in questa immagine",
    "engine.loading": "avvio del lettore",
    "engine.ready": "lettore pronto",

    "stat.images": "immagini",
    "stat.with": "con un codice",
    "stat.codes": "codici",
    "stat.unique": "univoci",
    "stat.without": "senza",
    "stat.unreadable": "illeggibili",

    "act.copyAll": "Copia tutto",
    "act.copy": "copia",
    "act.copied": "copiato",
    "act.download": "Scarica",
    "act.clear": "Svuota",
    "act.cancel": "Annulla",
    "act.clearAsk": "Buttare via tutti i risultati letti finora?",
    "act.open": "Apri",
    "act.saveContact": "Salva contatto",
    "act.reveal": "Mostra",
    "act.hide": "Nascondi",

    "rename.pattern": "Nuovo nome",
    "rename.code": "Il contenuto del codice",
    "rename.short": "La parte utile del codice",
    "rename.nameCode": "Nome originale + codice",
    "rename.keep": "Tieni i nomi originali",
    "rename.zip": "Scarica ZIP ({n})",
    "rename.none": "Nessuna immagine qui ha un codice da cui prendere il nome.",
    "rename.preview": "diventerà",

    "dupe.badge": "in {n} immagini",

    "err.advice": "Rifai la foto più da vicino, con il codice dritto e illuminato in modo uniforme — oppure ritaglia stretto attorno al codice e riprova.",

    "adv.title": "Preferisci il terminale? Usalo da lì",
    "adv.sub": "Lo stesso lettore come comando da mettere in uno script — e, mentre gira, questa pagina può anche ordinare e rinominare i file dove si trovano. Niente di tutto questo serve per leggere un codice.",
    "adv.localOn": "Una copia locale del programma sta rispondendo, quindi questa pagina può anche agire sui file su disco.",
    "adv.scan": "Analizza su disco",
    "adv.paths": "Cartelle o file, uno per riga",
    "adv.recreate": "Rigenera le immagini QR",
    "adv.organize": "Ordina in with_qr / without_qr",
    "adv.delete": "Elimina le immagini senza codice",
    "adv.confirmOrganize": "Spostare ogni immagine analizzata in una cartella with_qr / without_qr accanto ad essa?",
    "adv.confirmDelete": "Eliminare definitivamente ogni immagine analizzata che non ha un codice QR. Non si può annullare. Procedere?",
    "adv.scanFirst": "Prima analizza su disco.",

    "foot.privacy": "Le tue immagini non lasciano mai questo dispositivo.",

    "kind.url": "Link", "kind.wifi": "Rete Wi-Fi", "kind.contact": "Contatto",
    "kind.payment": "Pagamento", "kind.email": "Email", "kind.phone": "Numero di telefono",
    "kind.sms": "Messaggio", "kind.location": "Posizione", "kind.event": "Evento",
    "kind.crypto": "Indirizzo crypto", "kind.otp": "Configurazione a due fattori",
    "kind.reference": "Riferimento", "kind.text": "Testo",

    "f.site": "Sito", "f.reference": "Riferimento", "f.address": "Indirizzo",
    "f.network": "Rete", "f.security": "Sicurezza", "f.password": "Password",
    "f.name": "Nome", "f.phone": "Telefono", "f.email": "Email", "f.company": "Azienda",
    "f.payee": "Beneficiario", "f.iban": "IBAN", "f.amount": "Importo",
    "f.subject": "Oggetto", "f.message": "Messaggio", "f.number": "Numero",
    "f.latitude": "Latitudine", "f.longitude": "Longitudine",
    "f.title": "Titolo", "f.starts": "Inizio", "f.ends": "Fine", "f.where": "Dove",
    "f.account": "Account", "f.issuer": "Servizio", "f.algorithm": "Algoritmo",
    "f.secret": "Chiave segreta", "f.content": "Contenuto",
  };

  S.es = {
    "app.tagline": "lector QR",
    "nav.source": "Código fuente",

    "hero.title": "Lee, filtra y exporta todos los códigos QR de una carpeta de imágenes",
    "hero.sub": "Casi todos los lectores tratan una foto cada vez. Aquí arrastras una carpeta entera — o una sola foto, la cámara, o lo que tengas en el portapapeles — y obtienes todos los códigos de todas las imágenes de una vez.",
    "hero.trust": "No se sube nada — todo funciona en esta pestaña",

    "tab.images": "Imágenes",
    "tab.camera": "Cámara",

    "drop.title": "Arrastra aquí, o elige",
    "drop.sub": "Una foto, una carpeta, varias carpetas, o un zip",
    "drop.browse": "Elegir archivos",
    "drop.folder": "Elegir carpeta",
    "drop.paste": "Pegar una imagen",
    "drop.shoot": "Hacer una foto",

    "cam.off": "La cámara está apagada.<br>Apúntala a un código y lo lee en directo.",
    "cam.start": "Encender la cámara",
    "cam.stop": "Apagar la cámara",
    "cam.looking": "Buscando un código…",
    "cam.denied": "Permiso de cámara denegado.",
    "cam.none": "Aquí no hay ninguna cámara disponible.",
    "cam.read": "{n} códigos leídos hasta ahora.",

    "res.title": "Resultados",
    "res.private": "100% privado · funciona en tu navegador",
    "res.empty": "Los códigos que leas aparecerán aquí.",
    "res.nothing": "Ahí no hay nada que esta página pueda leer — solo imágenes.",
    "res.search": "Buscar dentro de los códigos…",
    "res.noMatch": "Ningún resultado coincide con la búsqueda.",
    "res.noCode": "Ningún código en esta imagen",
    "engine.loading": "iniciando el lector",
    "engine.ready": "lector listo",

    "stat.images": "imágenes",
    "stat.with": "con código",
    "stat.codes": "códigos",
    "stat.unique": "únicos",
    "stat.without": "sin código",
    "stat.unreadable": "ilegibles",

    "act.copyAll": "Copiar todo",
    "act.copy": "copiar",
    "act.copied": "copiado",
    "act.download": "Descargar",
    "act.clear": "Vaciar",
    "act.cancel": "Cancelar",
    "act.clearAsk": "¿Descartar todos los resultados leídos hasta ahora?",
    "act.open": "Abrir",
    "act.saveContact": "Guardar contacto",
    "act.reveal": "Mostrar",
    "act.hide": "Ocultar",

    "rename.pattern": "Nuevo nombre",
    "rename.code": "El contenido del código",
    "rename.short": "La parte útil del código",
    "rename.nameCode": "Nombre original + código",
    "rename.keep": "Mantener los nombres originales",
    "rename.zip": "Descargar ZIP ({n})",
    "rename.none": "Ninguna imagen tiene un código del que tomar el nombre.",
    "rename.preview": "se guardará como",

    "dupe.badge": "en {n} imágenes",

    "err.advice": "Repite la foto más de cerca, con el código recto y bien iluminado — o recorta justo alrededor del código e inténtalo de nuevo.",

    "adv.title": "¿Prefieres el terminal? Úsalo desde ahí",
    "adv.sub": "El mismo lector como un comando que puedes poner en un script — y, mientras se ejecuta, esta página también puede ordenar y renombrar los archivos donde están. Nada de esto hace falta para leer un código.",
    "adv.localOn": "Una copia local del programa está respondiendo, así que esta página también puede actuar sobre los archivos del disco.",
    "adv.scan": "Analizar en el disco",
    "adv.paths": "Carpetas o archivos, uno por línea",
    "adv.recreate": "Regenerar las imágenes QR",
    "adv.organize": "Ordenar en with_qr / without_qr",
    "adv.delete": "Borrar las imágenes sin código",
    "adv.confirmOrganize": "¿Mover cada imagen analizada a una carpeta with_qr / without_qr junto a ella?",
    "adv.confirmDelete": "Borrar definitivamente cada imagen analizada que no tenga código QR. No se puede deshacer. ¿Continuar?",
    "adv.scanFirst": "Analiza primero en el disco.",

    "foot.privacy": "Tus imágenes nunca salen de este dispositivo.",

    "kind.url": "Enlace", "kind.wifi": "Red Wi-Fi", "kind.contact": "Contacto",
    "kind.payment": "Pago", "kind.email": "Correo", "kind.phone": "Número de teléfono",
    "kind.sms": "Mensaje", "kind.location": "Ubicación", "kind.event": "Evento",
    "kind.crypto": "Dirección cripto", "kind.otp": "Configuración de dos factores",
    "kind.reference": "Referencia", "kind.text": "Texto",

    "f.site": "Sitio", "f.reference": "Referencia", "f.address": "Dirección",
    "f.network": "Red", "f.security": "Seguridad", "f.password": "Contraseña",
    "f.name": "Nombre", "f.phone": "Teléfono", "f.email": "Correo", "f.company": "Empresa",
    "f.payee": "Beneficiario", "f.iban": "IBAN", "f.amount": "Importe",
    "f.subject": "Asunto", "f.message": "Mensaje", "f.number": "Número",
    "f.latitude": "Latitud", "f.longitude": "Longitud",
    "f.title": "Título", "f.starts": "Empieza", "f.ends": "Termina", "f.where": "Dónde",
    "f.account": "Cuenta", "f.issuer": "Servicio", "f.algorithm": "Algoritmo",
    "f.secret": "Clave secreta", "f.content": "Contenido",
  };

  S.fr = {
    "app.tagline": "lecteur QR",
    "nav.source": "Code source",

    "hero.title": "Lisez, filtrez et exportez chaque code QR d'un dossier d'images",
    "hero.sub": "La plupart des lecteurs prennent une photo à la fois. Ici vous déposez un dossier entier — ou une seule photo, la caméra, ou ce qui est dans le presse-papiers — et vous obtenez chaque code de chaque image d'un coup.",
    "hero.trust": "Rien n'est envoyé — tout fonctionne dans cet onglet",

    "tab.images": "Images",
    "tab.camera": "Caméra",

    "drop.title": "Déposez ici, ou parcourez",
    "drop.sub": "Une photo, un dossier, plusieurs dossiers, ou un zip",
    "drop.browse": "Choisir des fichiers",
    "drop.folder": "Choisir un dossier",
    "drop.paste": "Coller une image",
    "drop.shoot": "Prendre une photo",

    "cam.off": "La caméra est éteinte.<br>Visez un code et il est lu en direct.",
    "cam.start": "Allumer la caméra",
    "cam.stop": "Éteindre la caméra",
    "cam.looking": "Recherche d'un code…",
    "cam.denied": "Autorisation de la caméra refusée.",
    "cam.none": "Aucune caméra disponible ici.",
    "cam.read": "{n} codes lus jusqu'ici.",

    "res.title": "Résultats",
    "res.private": "100% privé · fonctionne dans votre navigateur",
    "res.empty": "Les codes que vous lisez apparaîtront ici.",
    "res.nothing": "Il n'y a rien là que cette page sache lire — des images seulement.",
    "res.search": "Chercher dans les codes…",
    "res.noMatch": "Aucun résultat ne correspond à cette recherche.",
    "res.noCode": "Aucun code dans cette image",
    "engine.loading": "démarrage du lecteur",
    "engine.ready": "lecteur prêt",

    "stat.images": "images",
    "stat.with": "avec un code",
    "stat.codes": "codes",
    "stat.unique": "uniques",
    "stat.without": "sans",
    "stat.unreadable": "illisibles",

    "act.copyAll": "Tout copier",
    "act.copy": "copier",
    "act.copied": "copié",
    "act.download": "Télécharger",
    "act.clear": "Vider",
    "act.cancel": "Annuler",
    "act.clearAsk": "Jeter tous les résultats lus jusqu'ici ?",
    "act.open": "Ouvrir",
    "act.saveContact": "Enregistrer le contact",
    "act.reveal": "Afficher",
    "act.hide": "Masquer",

    "rename.pattern": "Nouveau nom",
    "rename.code": "Le contenu du code",
    "rename.short": "La partie utile du code",
    "rename.nameCode": "Nom d'origine + code",
    "rename.keep": "Garder les noms d'origine",
    "rename.zip": "Télécharger le ZIP ({n})",
    "rename.none": "Aucune image ici n'a de code dont tirer un nom.",
    "rename.preview": "sera enregistré comme",

    "dupe.badge": "dans {n} images",

    "err.advice": "Refaites la photo de plus près, le code bien droit et uniformément éclairé — ou recadrez au plus juste autour du code et réessayez.",

    "adv.title": "Vous préférez le terminal ? Utilisez-le là-bas",
    "adv.sub": "Le même lecteur sous forme de commande à mettre dans un script — et, pendant qu'il tourne, cette page peut aussi ranger et renommer les fichiers là où ils sont. Rien de tout cela n'est nécessaire pour lire un code.",
    "adv.localOn": "Une copie locale du programme répond, donc cette page peut aussi agir sur les fichiers du disque.",
    "adv.scan": "Analyser sur le disque",
    "adv.paths": "Dossiers ou fichiers, un par ligne",
    "adv.recreate": "Régénérer les images QR",
    "adv.organize": "Ranger dans with_qr / without_qr",
    "adv.delete": "Supprimer les images sans code",
    "adv.confirmOrganize": "Déplacer chaque image analysée dans un dossier with_qr / without_qr à côté d'elle ?",
    "adv.confirmDelete": "Supprimer définitivement chaque image analysée sans code QR. C'est irréversible. Continuer ?",
    "adv.scanFirst": "Analysez d'abord sur le disque.",

    "foot.privacy": "Vos images ne quittent jamais cet appareil.",

    "kind.url": "Lien", "kind.wifi": "Réseau Wi-Fi", "kind.contact": "Contact",
    "kind.payment": "Paiement", "kind.email": "E-mail", "kind.phone": "Numéro de téléphone",
    "kind.sms": "Message", "kind.location": "Position", "kind.event": "Événement",
    "kind.crypto": "Adresse crypto", "kind.otp": "Configuration à deux facteurs",
    "kind.reference": "Référence", "kind.text": "Texte",

    "f.site": "Site", "f.reference": "Référence", "f.address": "Adresse",
    "f.network": "Réseau", "f.security": "Sécurité", "f.password": "Mot de passe",
    "f.name": "Nom", "f.phone": "Téléphone", "f.email": "E-mail", "f.company": "Entreprise",
    "f.payee": "Bénéficiaire", "f.iban": "IBAN", "f.amount": "Montant",
    "f.subject": "Objet", "f.message": "Message", "f.number": "Numéro",
    "f.latitude": "Latitude", "f.longitude": "Longitude",
    "f.title": "Titre", "f.starts": "Début", "f.ends": "Fin", "f.where": "Lieu",
    "f.account": "Compte", "f.issuer": "Service", "f.algorithm": "Algorithme",
    "f.secret": "Clé secrète", "f.content": "Contenu",
  };

  S.de = {
    "app.tagline": "QR-Leser",
    "nav.source": "Quelltext",

    "hero.title": "Jeden QR-Code in einem Bilderordner lesen, filtern und exportieren",
    "hero.sub": "Die meisten Leser nehmen ein Bild nach dem anderen. Hier ziehen Sie einen ganzen Ordner hinein — oder ein einzelnes Foto, die Kamera, oder was in der Zwischenablage liegt — und bekommen jeden Code aus jedem Bild auf einmal.",
    "hero.trust": "Nichts wird hochgeladen — alles läuft in diesem Tab",

    "tab.images": "Bilder",
    "tab.camera": "Kamera",

    "drop.title": "Hierher ziehen, oder auswählen",
    "drop.sub": "Ein Foto, ein Ordner, mehrere Ordner, oder ein ZIP",
    "drop.browse": "Dateien wählen",
    "drop.folder": "Ordner wählen",
    "drop.paste": "Bild einfügen",
    "drop.shoot": "Foto aufnehmen",

    "cam.off": "Die Kamera ist aus.<br>Richten Sie sie auf einen Code, er wird sofort gelesen.",
    "cam.start": "Kamera einschalten",
    "cam.stop": "Kamera ausschalten",
    "cam.looking": "Suche nach einem Code…",
    "cam.denied": "Kamerazugriff wurde abgelehnt.",
    "cam.none": "Hier ist keine Kamera verfügbar.",
    "cam.read": "Bisher {n} Codes gelesen.",

    "res.title": "Ergebnisse",
    "res.private": "100% privat · läuft in Ihrem Browser",
    "res.empty": "Gelesene Codes erscheinen hier.",
    "res.nothing": "Da ist nichts, was diese Seite lesen kann — nur Bilder.",
    "res.search": "In den Codes suchen…",
    "res.noMatch": "Kein Ergebnis passt zu dieser Suche.",
    "res.noCode": "Kein Code in diesem Bild",
    "engine.loading": "Leser wird gestartet",
    "engine.ready": "Leser bereit",

    "stat.images": "Bilder",
    "stat.with": "mit Code",
    "stat.codes": "Codes",
    "stat.unique": "eindeutig",
    "stat.without": "ohne",
    "stat.unreadable": "unlesbar",

    "act.copyAll": "Alles kopieren",
    "act.copy": "kopieren",
    "act.copied": "kopiert",
    "act.download": "Herunterladen",
    "act.clear": "Leeren",
    "act.cancel": "Abbrechen",
    "act.clearAsk": "Alle bisher gelesenen Ergebnisse verwerfen?",
    "act.open": "Öffnen",
    "act.saveContact": "Kontakt speichern",
    "act.reveal": "Anzeigen",
    "act.hide": "Verbergen",

    "rename.pattern": "Neuer Name",
    "rename.code": "Der Inhalt des Codes",
    "rename.short": "Der brauchbare Teil des Codes",
    "rename.nameCode": "Ursprünglicher Name + Code",
    "rename.keep": "Ursprüngliche Namen behalten",
    "rename.zip": "ZIP herunterladen ({n})",
    "rename.none": "Kein Bild hier hat einen Code, aus dem ein Name werden könnte.",
    "rename.preview": "wird gespeichert als",

    "dupe.badge": "in {n} Bildern",

    "err.advice": "Machen Sie das Foto näher, mit dem Code gerade und gleichmäßig beleuchtet — oder schneiden Sie eng um den Code zu und versuchen Sie es erneut.",

    "adv.title": "Lieber das Terminal? Dann von dort",
    "adv.sub": "Derselbe Leser als Befehl für ein Skript — und während er läuft, kann diese Seite die Dateien auch dort sortieren und umbenennen, wo sie liegen. Nichts davon wird gebraucht, um einen Code zu lesen.",
    "adv.localOn": "Eine lokale Kopie des Programms antwortet, diese Seite kann also auch auf Dateien auf der Festplatte wirken.",
    "adv.scan": "Auf der Festplatte suchen",
    "adv.paths": "Ordner oder Dateien, einer pro Zeile",
    "adv.recreate": "QR-Bilder neu erzeugen",
    "adv.organize": "In with_qr / without_qr einsortieren",
    "adv.delete": "Bilder ohne Code löschen",
    "adv.confirmOrganize": "Jedes gefundene Bild in einen with_qr / without_qr Ordner daneben verschieben?",
    "adv.confirmDelete": "Jedes gefundene Bild ohne QR-Code endgültig löschen. Das lässt sich nicht rückgängig machen. Fortfahren?",
    "adv.scanFirst": "Suchen Sie zuerst auf der Festplatte.",

    "foot.privacy": "Ihre Bilder verlassen dieses Gerät nie.",

    "kind.url": "Link", "kind.wifi": "WLAN-Netz", "kind.contact": "Kontakt",
    "kind.payment": "Zahlung", "kind.email": "E-Mail", "kind.phone": "Telefonnummer",
    "kind.sms": "Nachricht", "kind.location": "Ort", "kind.event": "Termin",
    "kind.crypto": "Krypto-Adresse", "kind.otp": "Zwei-Faktor-Einrichtung",
    "kind.reference": "Referenz", "kind.text": "Text",

    "f.site": "Seite", "f.reference": "Referenz", "f.address": "Adresse",
    "f.network": "Netz", "f.security": "Sicherheit", "f.password": "Passwort",
    "f.name": "Name", "f.phone": "Telefon", "f.email": "E-Mail", "f.company": "Firma",
    "f.payee": "Empfänger", "f.iban": "IBAN", "f.amount": "Betrag",
    "f.subject": "Betreff", "f.message": "Nachricht", "f.number": "Nummer",
    "f.latitude": "Breite", "f.longitude": "Länge",
    "f.title": "Titel", "f.starts": "Beginn", "f.ends": "Ende", "f.where": "Wo",
    "f.account": "Konto", "f.issuer": "Dienst", "f.algorithm": "Algorithmus",
    "f.secret": "Geheimer Schlüssel", "f.content": "Inhalt",
  };

  S.pt = {
    "app.tagline": "leitor QR",
    "nav.source": "Código fonte",

    "hero.title": "Leia, filtre e exporte todos os códigos QR de uma pasta de imagens",
    "hero.sub": "Quase todos os leitores tratam uma foto de cada vez. Aqui arrasta uma pasta inteira — ou uma só foto, a câmara, ou o que estiver na área de transferência — e obtém todos os códigos de todas as imagens de uma vez.",
    "hero.trust": "Nada é enviado — funciona tudo neste separador",

    "tab.images": "Imagens",
    "tab.camera": "Câmara",

    "drop.title": "Arraste para aqui, ou escolha",
    "drop.sub": "Uma foto, uma pasta, várias pastas, ou um zip",
    "drop.browse": "Escolher ficheiros",
    "drop.folder": "Escolher pasta",
    "drop.paste": "Colar uma imagem",
    "drop.shoot": "Tirar uma foto",

    "cam.off": "A câmara está desligada.<br>Aponte para um código e ele é lido em direto.",
    "cam.start": "Ligar a câmara",
    "cam.stop": "Desligar a câmara",
    "cam.looking": "À procura de um código…",
    "cam.denied": "Permissão da câmara recusada.",
    "cam.none": "Não há câmara disponível aqui.",
    "cam.read": "{n} códigos lidos até agora.",

    "res.title": "Resultados",
    "res.private": "100% privado · funciona no seu navegador",
    "res.empty": "Os códigos que ler aparecem aqui.",
    "res.nothing": "Não há ali nada que esta página saiba ler — só imagens.",
    "res.search": "Procurar dentro dos códigos…",
    "res.noMatch": "Nenhum resultado corresponde a essa procura.",
    "res.noCode": "Nenhum código nesta imagem",
    "engine.loading": "a iniciar o leitor",
    "engine.ready": "leitor pronto",

    "stat.images": "imagens",
    "stat.with": "com código",
    "stat.codes": "códigos",
    "stat.unique": "únicos",
    "stat.without": "sem código",
    "stat.unreadable": "ilegíveis",

    "act.copyAll": "Copiar tudo",
    "act.copy": "copiar",
    "act.copied": "copiado",
    "act.download": "Descarregar",
    "act.clear": "Limpar",
    "act.cancel": "Cancelar",
    "act.clearAsk": "Deitar fora todos os resultados lidos até agora?",
    "act.open": "Abrir",
    "act.saveContact": "Guardar contacto",
    "act.reveal": "Mostrar",
    "act.hide": "Esconder",

    "rename.pattern": "Novo nome",
    "rename.code": "O conteúdo do código",
    "rename.short": "A parte útil do código",
    "rename.nameCode": "Nome original + código",
    "rename.keep": "Manter os nomes originais",
    "rename.zip": "Descarregar ZIP ({n})",
    "rename.none": "Nenhuma imagem aqui tem um código de onde tirar o nome.",
    "rename.preview": "será guardado como",

    "dupe.badge": "em {n} imagens",

    "err.advice": "Repita a foto mais de perto, com o código direito e bem iluminado — ou corte justo à volta do código e tente de novo.",

    "adv.title": "Prefere o terminal? Use-o a partir daí",
    "adv.sub": "O mesmo leitor como um comando para pôr num script — e, enquanto corre, esta página pode também arrumar e renomear os ficheiros onde estão. Nada disto é preciso para ler um código.",
    "adv.localOn": "Uma cópia local do programa está a responder, por isso esta página pode também agir sobre os ficheiros no disco.",
    "adv.scan": "Analisar no disco",
    "adv.paths": "Pastas ou ficheiros, um por linha",
    "adv.recreate": "Recriar as imagens QR",
    "adv.organize": "Arrumar em with_qr / without_qr",
    "adv.delete": "Apagar as imagens sem código",
    "adv.confirmOrganize": "Mover cada imagem analisada para uma pasta with_qr / without_qr ao lado dela?",
    "adv.confirmDelete": "Apagar definitivamente cada imagem analisada sem código QR. Não se pode desfazer. Continuar?",
    "adv.scanFirst": "Analise primeiro no disco.",

    "foot.privacy": "As suas imagens nunca saem deste dispositivo.",

    "kind.url": "Ligação", "kind.wifi": "Rede Wi-Fi", "kind.contact": "Contacto",
    "kind.payment": "Pagamento", "kind.email": "Email", "kind.phone": "Número de telefone",
    "kind.sms": "Mensagem", "kind.location": "Localização", "kind.event": "Evento",
    "kind.crypto": "Endereço cripto", "kind.otp": "Configuração de dois fatores",
    "kind.reference": "Referência", "kind.text": "Texto",

    "f.site": "Site", "f.reference": "Referência", "f.address": "Endereço",
    "f.network": "Rede", "f.security": "Segurança", "f.password": "Palavra-passe",
    "f.name": "Nome", "f.phone": "Telefone", "f.email": "Email", "f.company": "Empresa",
    "f.payee": "Beneficiário", "f.iban": "IBAN", "f.amount": "Montante",
    "f.subject": "Assunto", "f.message": "Mensagem", "f.number": "Número",
    "f.latitude": "Latitude", "f.longitude": "Longitude",
    "f.title": "Título", "f.starts": "Início", "f.ends": "Fim", "f.where": "Onde",
    "f.account": "Conta", "f.issuer": "Serviço", "f.algorithm": "Algoritmo",
    "f.secret": "Chave secreta", "f.content": "Conteúdo",
  };

  global.QR_STRINGS = S;
  global.QR_LANG_NAMES = NAMES;
})(typeof globalThis !== "undefined" ? globalThis : this);
