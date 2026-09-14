package service

import "strings"

/*
「그 티커는 흔한 영어 낱말이다」를 판정한다.

이 파일이 있는 이유는 손으로 관리하던 불용어 목록이 **매번 새 오탐이 생긴 뒤에야
한 낱말씩 늘어났기** 때문이다. 08-24에 GOLD·BOND·HERE·HONG·OPEN·FIVE 여섯을 넣었는데,
09-03 실측(최근 US 기사 1,000건)에서 같은 종류가 그대로 스물두 개 더 나왔다:

	MOVE 30 · AMID 20 · ALTO 18 · PALO 18 · HIGH 17 · LIVE 15 · TECH 14 ·
	GAIN 13 · BEAT 12 · CORP 11 · POST 8 · HOLD 7 · COOK 5 · FOUR 5 ·
	AWAY 4 · LINE 4 · LOVE 4 · PATH 4 · ASIA 3 · BILL 3 · BULL 3 · DIPS 3

「stocks fall **amid** rate fears」가 ARGENT MID CAP ETF 기사가 되고,
「**high** yield」가 SIMPLIFY ENHANCED INCOME ETF 기사가 된다. 한 낱말씩 쫓아가는 한
이 목록은 영원히 뒤처진다 — **낱말의 집합은 유한하니 미리 적는다.**

티커는 최대 5자이고 이름 인덱스는 4자부터 받으므로, 실제로 필요한 것은 **4~5자
영어 낱말**뿐이다. 그 범위는 다 적을 수 있다. 여기 있는 티커는 지워지는 것이 아니라
**「$AMID」·「NASDAQ: AMID」처럼 티커임을 밝힌 표기로만** 붙는다
(`marketdata.ExtractCashtags` · `ExtractExchangeTickers`).

경계선의 판단 둘:

  - **회사 이름이 낱말이어도 이름 쪽은 막지 않는다.** Apple·Target은 이름으로
    매치되고, 여기 목록은 **티커 문자열**에만 적용된다. 다만 그 이름이 금융
    기사에서 낱말로 훨씬 자주 쓰이면 `usNameStopwords`가 따로 막는다(target·nasdaq).
  - **너무 넓게 막지 않는다.** META·VISA·FORD처럼 「낱말이 아닌 고유명사」는 넣지
    않는다. 넣으면 진짜 기사를 놓친다.
*/

// commonEnglishTickerWords — 4~5자 영어 낱말. 티커가 이것과 같으면 자유 본문에서
// 매치하지 않는다. 소문자로 비교한다.
var commonEnglishTickerWords = map[string]struct{}{}

func init() {
	// 뭉텅이로 적어 두고 init에서 집합으로 만든다 — 목록을 늘릴 때 줄만 고치면 된다.
	for _, group := range [][]string{
		// 실측으로 오탐을 만든 것들 (2026-08-24 · 09-03)
		{"gold", "bond", "open", "here", "hong", "five", "move", "amid", "alto",
			"palo", "high", "live", "tech", "gain", "beat", "corp", "post", "hold",
			"cook", "four", "away", "line", "love", "path", "asia", "bill", "bull",
			"dips", "cash", "fund", "real", "core", "next", "best", "safe", "wall",
			"main", "peak", "wave", "boss", "hope", "well"},
		// 금융 기사에서 특히 잦은 낱말
		{"bear", "bears", "bulls", "buys", "sell", "sells", "loss", "risk", "rate",
			"rates", "debt", "cost", "cap", "caps", "fees", "flow", "fund", "gains",
			"held", "hike", "hikes", "lows", "peer", "peers", "plan", "plans",
			"rise", "rose", "sale", "sales", "save", "size", "term", "tier", "vote",
			"yield", "yields", "worth", "trade", "trend", "value", "wages", "banks",
			"bonds", "chips", "deals", "drop", "drops", "earns", "funds", "goods",
			"asset", "bonus", "boost", "brand", "buyer", "cards", "china", "close",
			"crash", "curve", "cycle", "index", "issue", "level", "money", "north",
			"offer", "order", "price", "range", "ratio", "share", "shift", "short",
			"south", "spend", "stake", "stock", "store", "surge", "total", "wells"},
		// 일반 4~5자 낱말 (헤드라인에 흔한 것 위주)
		{"able", "also", "back", "base", "bank", "been", "bell", "belt", "bird",
			"blue", "boat", "body", "book", "born", "both", "call", "came", "care",
			"case", "cell", "chip", "city", "club", "code", "cold", "come", "cool",
			"crew", "dark", "data", "date", "days", "dead", "deal", "deep", "does",
			"done", "door", "down", "draw", "each", "earn", "ease", "east", "easy",
			"edge", "else", "even", "ever", "eyes", "face", "fact", "fair", "fall",
			"farm", "fast", "fear", "feed", "feel", "file", "fill", "film", "find",
			"fine", "fire", "firm", "fish", "flat", "food", "foot", "form", "free",
			"from", "fuel", "full", "game", "gate", "gave", "gear", "gets", "gift",
			"girl", "give", "glass", "goal", "goes", "gone", "good", "grew", "grid",
			"grow", "half", "hall", "hand", "hard", "harm", "have", "head", "heat",
			"held", "help", "hero", "hide", "hill", "hire", "hits", "home", "hour",
			"huge", "idea", "into", "iron", "item", "join", "jump", "just", "keep",
			"kept", "keys", "kids", "kind", "king", "know", "labs", "lack", "lake",
			"land", "last", "late", "lead", "leaf", "left", "legs", "less", "life",
			"lift", "like", "list", "lite", "load", "lock", "long", "look", "lost",
			"lots", "loud", "luck", "made", "mail", "make", "male", "mall", "many",
			"maps", "mark", "mass", "meal", "mean", "meat", "meet", "mile", "milk",
			"mind", "mine", "miss", "mode", "mood", "moon", "more", "most", "much",
			"must", "name", "near", "neck", "need", "news", "nice", "nine", "none",
			"note", "noon", "okay", "once", "ones", "only", "onto", "over", "pace",
			"pack", "page", "paid", "pain", "pair", "park", "part", "pass", "past",
			"pay", "peak", "pick", "pill", "pipe", "plus", "poll", "pool", "poor",
			"pope", "port", "pull", "pure", "push", "race", "rain", "rank", "rare",
			"read", "ride", "ring", "road", "rock", "role", "roll", "room", "root",
			"rose", "rule", "runs", "rush", "said", "salt", "same", "sand", "says",
			"seat", "seek", "seem", "seen", "self", "sent", "sets", "shed", "ship",
			"shop", "shot", "show", "shut", "sick", "side", "sign", "site", "skin",
			"slow", "snap", "snow", "soft", "soil", "sold", "sole", "some", "song",
			"soon", "sort", "soul", "spot", "star", "stay", "step", "stop", "such",
			"suit", "sure", "take", "talk", "tall", "tape", "task", "team", "tell",
			"tend", "test", "text", "than", "that", "them", "then", "they", "thin",
			"this", "thus", "tied", "time", "tiny", "told", "tone", "took", "tool",
			"tops", "torn", "tour", "town", "trip", "true", "turn", "twin", "type",
			"unit", "upon", "used", "user", "vast", "very", "view", "vote", "wait",
			"walk", "want", "ward", "warm", "warn", "wash", "watch", "ways", "weak",
			"wear", "week", "went", "were", "west", "what", "when", "whom", "wide",
			"wild", "will", "wind", "wine", "wing", "wins", "wire", "wise", "wish",
			"with", "wood", "word", "work", "wrap", "yard", "year", "your", "zero",
			"zone"},
		{"about", "above", "added", "after", "again", "ahead", "alert", "alone",
			"along", "among", "apple", "areas", "avoid", "aware", "began", "being",
			"below", "birth", "black", "blame", "blend", "block", "board", "break",
			"bring", "broad", "broke", "brown", "build", "built", "buyer", "cable",
			"carry", "catch", "cause", "chain", "chair", "chart", "cheap", "check",
			"chief", "child", "civil", "claim", "class", "clean", "clear", "click",
			"climb", "clock", "cloud", "coach", "coast", "color", "count", "court",
			"cover", "craft", "crazy", "cream", "crime", "crisis", "cross", "crowd",
			"daily", "dance", "dated", "death", "delay", "dense", "depth", "doubt",
			"dozen", "draft", "drama", "dream", "dress", "drink", "drive", "drove",
			"eager", "early", "earth", "eight", "elite", "empty", "ended", "enemy",
			"enjoy", "enter", "entry", "equal", "error", "event", "every", "exact",
			"exist", "extra", "faced", "faith", "false", "fault", "favor", "field",
			"fifth", "fight", "final", "first", "fixed", "flash", "fleet", "flesh",
			"float", "floor", "focus", "force", "forth", "forum", "found", "frame",
			"fraud", "fresh", "front", "fruit", "fully", "funny", "giant", "given",
			"glass", "globe", "going", "grace", "grade", "grand", "grant", "grass",
			"great", "green", "gross", "group", "grown", "guard", "guess", "guest",
			"guide", "happy", "heart", "heavy", "hence", "hotel", "house", "human",
			"ideal", "image", "imply", "inner", "input", "irony", "japan", "joint",
			"judge", "known", "labor", "large", "laser", "later", "laugh", "layer",
			"learn", "lease", "least", "leave", "legal", "lemon", "level", "light",
			"limit", "linked", "local", "logic", "loose", "lower", "lucky", "lunch",
			"macro", "magic", "major", "maker", "March", "match", "maybe", "mayor",
			"meant", "media", "metal", "meter", "might", "minor", "minus", "mixed",
			"model", "month", "moral", "motor", "mount", "mouse", "mouth", "movie",
			"music", "naked", "nerve", "never", "newly", "night", "noise", "north",
			"noted", "novel", "nurse", "occur", "ocean", "often", "older", "olive",
			"onion", "opens", "opera", "orbit", "other", "ought", "outer", "owned",
			"owner", "paint", "panel", "paper", "party", "pause", "peace", "phase",
			"phone", "photo", "piece", "pilot", "pitch", "place", "plain", "plane",
			"plant", "plate", "point", "polls", "pound", "power", "press", "pride",
			"prime", "print", "prior", "prize", "probe", "proof", "proud", "prove",
			"pulse", "punch", "pupil", "queen", "quick", "quiet", "quite", "quote",
			"radio", "raise", "rally", "rapid", "reach", "ready", "realm", "rebel",
			"refer", "relax", "renew", "reply", "rider", "ridge", "rifle", "right",
			"rigid", "rival", "river", "robot", "rocky", "roman", "rough", "round",
			"route", "royal", "rural", "salad", "scale", "scene", "scope", "score",
			"scrap", "sense", "serve", "setup", "seven", "shall", "shame", "shape",
			"sharp", "sheet", "shelf", "shell", "shine", "shirt", "shock", "shoot",
			"shore", "sight", "silly", "since", "sixth", "sixty", "skill", "sleep",
			"slice", "slide", "slope", "small", "smart", "smell", "smile", "smoke",
			"snake", "solar", "solid", "solve", "sorry", "sound", "space", "spare",
			"speak", "speed", "spell", "spend", "spent", "spike", "spine", "split",
			"spoke", "sport", "spray", "squad", "stack", "staff", "stage", "stand",
			"start", "state", "stays", "steam", "steel", "steep", "steer", "stern",
			"stick", "stiff", "still", "stone", "stood", "storm", "story", "strip",
			"stuck", "study", "stuff", "style", "sugar", "suite", "sunny", "super",
			"sweet", "swift", "swing", "table", "taken", "taste", "taxes", "teach",
			"tears", "teeth", "tempo", "tends", "tenth", "terms", "texas", "thank",
			"theft", "their", "theme", "there", "these", "thick", "thing", "think",
			"third", "those", "three", "threw", "throw", "thumb", "tight", "times",
			"tired", "title", "today", "token", "tooth", "topic", "total", "touch",
			"tough", "tower", "toxic", "track", "tract", "trail", "train", "trait",
			"trash", "treat", "trial", "tribe", "trick", "tried", "tries", "truck",
			"truly", "trust", "truth", "twice", "twist", "under", "union", "unite",
			"unity", "until", "upper", "upset", "urban", "urged", "usage", "using",
			"usual", "valid", "vapor", "vault", "venue", "verse", "video", "views",
			"villa", "vinyl", "viral", "virus", "visit", "vital", "vocal", "voice",
			"votes", "wages", "wagon", "waste", "watch", "water", "weigh", "weird",
			"wheat", "wheel", "where", "which", "while", "white", "whole", "whose",
			"width", "woman", "women", "words", "works", "world", "worry", "worse",
			"worst", "would", "wound", "write", "wrong", "wrote", "yield", "young",
			"yours", "youth"},
	} {
		for _, w := range group {
			commonEnglishTickerWords[strings.ToLower(w)] = struct{}{}
		}
	}
}

/*
isWordLikeTicker는 이 티커가 **자유 본문에서 매치하면 안 되는** 티커인지 말한다.

세 글자 이하는 항상 true다 — `usTickerMinFreeTextLen`이 이미 막고 있지만, 이 함수를
보는 쪽이 그 사실을 다시 확인하지 않아도 되게 여기서도 말한다. V·MA·F 같은 티커가
자유 본문에서 매치되면 기사 절반에 붙는다.
*/
func isWordLikeTicker(sym string) bool {
	sym = strings.ToLower(strings.TrimSpace(sym))
	if sym == "" {
		return true
	}
	if len(sym) < usTickerMinFreeTextLen {
		return true
	}
	_, ok := commonEnglishTickerWords[sym]
	return ok
}
