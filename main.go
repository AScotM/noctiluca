package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Vec struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type Agent struct {
	P      Vec        `json:"position"`
	V      Vec        `json:"velocity"`
	Phase  float64    `json:"phase"`
	Omega  float64    `json:"omega"`
	Energy float64    `json:"energy"`
	Hue    float64    `json:"hue"`
	Age    int        `json:"age"`
	Genome [4]float64 `json:"genome"`
}
type Config struct {
	Mode     string  `json:"mode"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Count    int     `json:"count"`
	Seed     int64   `json:"seed"`
	Dt       float64 `json:"dt"`
	Coupling float64 `json:"coupling"`
	Radius   float64 `json:"radius"`
	Trail    float64 `json:"trail"`
	Flow     float64 `json:"flow"`
	Mutation float64 `json:"mutation"`
	Epoch    int     `json:"epoch"`
}
type State struct {
	Version int       `json:"version"`
	Config  Config    `json:"config"`
	Tick    int       `json:"tick"`
	Agents  []Agent   `json:"agents"`
	Field   []float64 `json:"field"`
	Births  int       `json:"births"`
	Deaths  int       `json:"deaths"`
}
type Metrics struct {
	Tick       int     `json:"tick"`
	Time       float64 `json:"time"`
	Population int     `json:"population"`
	Order      float64 `json:"phase_order"`
	MeanEnergy float64 `json:"mean_energy"`
	MeanSpeed  float64 `json:"mean_speed"`
	Luminosity float64 `json:"luminosity"`
	Births     int     `json:"births"`
	Deaths     int     `json:"deaths"`
}
type Options struct {
	Frames    int
	FPS       int
	Output    string
	Snapshot  string
	Input     string
	Export    string
	CSV       string
	NoColor   bool
	Quiet     bool
	Palette   string
	Render    string
	SaveEvery int
}

func clamp(x, a, b float64) float64 { return math.Max(a, math.Min(b, x)) }
func wrap(x, max float64) float64 {
	x = math.Mod(x, max)
	if x < 0 {
		x += max
	}
	return x
}
func dist(a, b Vec, w, h float64) (float64, float64, float64) {
	dx := b.X - a.X
	dy := b.Y - a.Y
	if dx > w/2 {
		dx -= w
	}
	if dx < -w/2 {
		dx += w
	}
	if dy > h/2 {
		dy -= h
	}
	if dy < -h/2 {
		dy += h
	}
	return dx, dy, math.Hypot(dx, dy)
}
func randomAgent(r *rand.Rand, c Config) Agent {
	a := Agent{P: Vec{r.Float64() * float64(c.Width), r.Float64() * float64(c.Height)}, V: Vec{(r.Float64() - .5) * .6, (r.Float64() - .5) * .6}, Phase: r.Float64() * 2 * math.Pi, Omega: .4 + r.Float64()*1.4, Energy: .5 + r.Float64()*.5, Hue: r.Float64() * 360}
	for i := range a.Genome {
		a.Genome[i] = .5 + r.Float64()
	}
	return a
}
func newState(c Config) *State {
	r := rand.New(rand.NewSource(c.Seed))
	s := &State{Version: 1, Config: c, Agents: make([]Agent, c.Count), Field: make([]float64, c.Width*c.Height)}
	for i := range s.Agents {
		s.Agents[i] = randomAgent(r, c)
	}
	return s
}
func fieldIndex(s *State, x, y int) int {
	x = (x%s.Config.Width + s.Config.Width) % s.Config.Width
	y = (y%s.Config.Height + s.Config.Height) % s.Config.Height
	return y*s.Config.Width + x
}
func step(s *State, r *rand.Rand) {
	c := s.Config
	w, h := float64(c.Width), float64(c.Height)
	next := make([]Agent, 0, len(s.Agents)+2)
	for i, a := range s.Agents {
		var ax, ay, px, py, sync float64
		neighbors := 0
		for j, b := range s.Agents {
			if i == j {
				continue
			}
			dx, dy, d := dist(a.P, b.P, w, h)
			if d < .001 || d > c.Radius {
				continue
			}
			neighbors++
			weight := 1 - d/c.Radius
			sync += math.Sin(b.Phase-a.Phase) * weight
			ax += (b.V.X - a.V.X) * weight
			ay += (b.V.Y - a.V.Y) * weight
			px -= dx / (d*d + .3) * weight
			py -= dy / (d*d + .3) * weight
		}
		if neighbors > 0 {
			inv := 1 / float64(neighbors)
			ax *= inv
			ay *= inv
			px *= inv
			py *= inv
			sync *= inv
		}
		fx, fy := 0.0, 0.0
		switch c.Mode {
		case "vortex":
			dx, dy, _ := dist(Vec{w / 2, h / 2}, a.P, w, h)
			fx = -dy / math.Max(w, h)
			fy = dx / math.Max(w, h)
		case "nebula":
			fx = math.Sin(a.P.Y*.22+float64(s.Tick)*.012) * .5
			fy = math.Cos(a.P.X*.19-float64(s.Tick)*.009) * .5
		case "river":
			fx = .8 + math.Sin(a.P.Y*.35+float64(s.Tick)*.01)*.3
			fy = math.Sin(a.P.X*.15) * .15
		case "constellation":
			dx, dy, d := dist(a.P, Vec{w/2 + math.Sin(float64(s.Tick)*.005)*w*.2, h / 2}, w, h)
			if d > 0 {
				fx = dx / d
				fy = dy / d
			}
		case "eclipse":
			dx, dy, d := dist(a.P, Vec{w / 2, h / 2}, w, h)
			if d > 0 {
				fx = -dx / d
				fy = -dy / d
			}
		}
		noiseX := (r.Float64() - .5) * .09
		noiseY := (r.Float64() - .5) * .09
		a.V.X = clamp((a.V.X+(.24*ax+.7*px+c.Flow*fx+noiseX)*c.Dt)*.985, -1.8, 1.8)
		a.V.Y = clamp((a.V.Y+(.24*ay+.7*py+c.Flow*fy+noiseY)*c.Dt)*.985, -1.8, 1.8)
		speed := math.Hypot(a.V.X, a.V.Y)
		if speed > 1.35 {
			a.V.X *= 1.35 / speed
			a.V.Y *= 1.35 / speed
		}
		a.P.X = wrap(a.P.X+a.V.X*c.Dt, w)
		a.P.Y = wrap(a.P.Y+a.V.Y*c.Dt, h)
		a.Phase = wrap(a.Phase+(a.Omega+c.Coupling*sync)*c.Dt, 2*math.Pi)
		a.Energy = clamp(a.Energy+.002*c.Dt-.0008*speed*c.Dt+.002*(r.Float64()-.5), .15, 1)
		a.Age++
		if c.Epoch > 0 && a.Age > c.Epoch && r.Float64() < .003 {
			s.Deaths++
			continue
		}
		next = append(next, a)
	}
	if c.Epoch > 0 && s.Tick > 0 && s.Tick%c.Epoch == 0 && len(next) > 0 && len(next) < c.Count*2 {
		parent := next[r.Intn(len(next))]
		child := parent
		child.Age = 0
		child.P.X = wrap(child.P.X+(r.Float64()-.5)*3, w)
		child.P.Y = wrap(child.P.Y+(r.Float64()-.5)*3, h)
		child.Phase = wrap(child.Phase+r.Float64(), 2*math.Pi)
		child.Energy = .65
		for i := range child.Genome {
			child.Genome[i] = clamp(child.Genome[i]+(r.Float64()-.5)*c.Mutation, .1, 2)
		}
		child.Omega = clamp(child.Omega+(r.Float64()-.5)*c.Mutation, .1, 3)
		next = append(next, child)
		s.Births++
	}
	s.Agents = next
	for i := range s.Field {
		s.Field[i] *= c.Trail
	}
	for _, a := range s.Agents {
		x, y := int(a.P.X), int(a.P.Y)
		brightness := a.Energy * (.35 + .65*math.Pow(.5+.5*math.Sin(a.Phase), 2))
		for oy := -2; oy <= 2; oy++ {
			for ox := -2; ox <= 2; ox++ {
				d2 := float64(ox*ox + oy*oy)
				idx := fieldIndex(s, x+ox, y+oy)
				s.Field[idx] = clamp(s.Field[idx]+brightness*math.Exp(-d2/2.0)*.65, 0, 2.5)
			}
		}
	}
	s.Tick++
}
func metrics(s *State) Metrics {
	m := Metrics{Tick: s.Tick, Time: float64(s.Tick) * s.Config.Dt, Population: len(s.Agents), Births: s.Births, Deaths: s.Deaths}
	var sn, cs float64
	for _, a := range s.Agents {
		sn += math.Sin(a.Phase)
		cs += math.Cos(a.Phase)
		m.MeanEnergy += a.Energy
		m.MeanSpeed += math.Hypot(a.V.X, a.V.Y)
	}
	if len(s.Agents) > 0 {
		n := float64(len(s.Agents))
		m.Order = math.Hypot(sn, cs) / n
		m.MeanEnergy /= n
		m.MeanSpeed /= n
	}
	for _, v := range s.Field {
		m.Luminosity += v
	}
	m.Luminosity /= float64(len(s.Field))
	return m
}
func rgb(v float64, palette string) (int, int, int) {
	v = clamp(v/1.7, 0, 1)
	var r, g, b float64
	switch palette {
	case "ember":
		r = 255 * v
		g = 190 * math.Pow(v, 1.8)
		b = 75 * math.Pow(v, 3)
	case "aurora":
		r = 100 * math.Pow(v, 2)
		g = 255 * v
		b = 230 * math.Sqrt(v)
	case "violet":
		r = 220 * v
		g = 90 * math.Pow(v, 2)
		b = 255 * math.Sqrt(v)
	case "mono":
		r = 255 * v
		g = r
		b = r
	default:
		r = 45 * math.Pow(v, 2)
		g = 195 * v
		b = 255 * math.Sqrt(v)
	}
	return int(r), int(g), int(b)
}
func render(s *State, opt Options) string {
	var b strings.Builder
	if opt.Render == "braille" {
		for y := 0; y < s.Config.Height; y += 4 {
			for x := 0; x < s.Config.Width; x += 2 {
				bits := []struct {
					dx, dy int
					bit    rune
				}{{0, 0, 0x1}, {0, 1, 0x2}, {0, 2, 0x4}, {0, 3, 0x40}, {1, 0, 0x8}, {1, 1, 0x10}, {1, 2, 0x20}, {1, 3, 0x80}}
				var ch rune = 0x2800
				var total float64
				for _, p := range bits {
					v := s.Field[fieldIndex(s, x+p.dx, y+p.dy)]
					total += v
					if v > .15 {
						ch += p.bit
					}
				}
				if opt.NoColor {
					b.WriteRune(ch)
				} else {
					r, g, bl := rgb(total/8, opt.Palette)
					fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm%c", r, g, bl, ch)
				}
			}
			b.WriteString("\x1b[0m\n")
		}
	} else {
		ramp := []rune(" .·:+=*#%@")
		for y := 0; y < s.Config.Height; y++ {
			for x := 0; x < s.Config.Width; x++ {
				v := s.Field[fieldIndex(s, x, y)]
				idx := int(clamp(v/1.8, 0, 1) * float64(len(ramp)-1))
				if opt.NoColor {
					b.WriteRune(ramp[idx])
				} else {
					r, g, bl := rgb(v, opt.Palette)
					fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm%c", r, g, bl, ramp[idx])
				}
			}
			b.WriteString("\x1b[0m\n")
		}
	}
	m := metrics(s)
	fmt.Fprintf(&b, "NOCTILUCA  %s  tick %d  organisms %d  coherence %.3f  luminance %.4f  births %d  deaths %d\n", s.Config.Mode, m.Tick, m.Population, m.Order, m.Luminosity, m.Births, m.Deaths)
	return b.String()
}
func save(path string, s *State) error {
	if path == "" {
		return nil
	}
	dir := filepath.Dir(path)
	f, e := os.CreateTemp(dir, ".noctiluca-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(0600); e != nil {
		f.Close()
		return e
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if e = enc.Encode(s); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
func load(path string) (*State, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	var s State
	e = json.NewDecoder(f).Decode(&s)
	if e != nil {
		return nil, e
	}
	if s.Version != 1 || s.Config.Width < 4 || s.Config.Height < 4 || len(s.Field) != s.Config.Width*s.Config.Height || len(s.Agents) > 100000 {
		return nil, errors.New("invalid or unsupported snapshot")
	}
	return &s, nil
}
func exportSVG(path string, s *State, palette string) error {
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()
	fmt.Fprintf(w, "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 %d %d\" width=\"%d\" height=\"%d\"><rect width=\"100%%\" height=\"100%%\" fill=\"#030711\"/>", s.Config.Width*8, s.Config.Height*8, s.Config.Width*8, s.Config.Height*8)
	for y := 0; y < s.Config.Height; y++ {
		for x := 0; x < s.Config.Width; x++ {
			v := s.Field[fieldIndex(s, x, y)]
			if v < .03 {
				continue
			}
			r, g, b := rgb(v, palette)
			fmt.Fprintf(w, "<rect x=\"%d\" y=\"%d\" width=\"8\" height=\"8\" fill=\"#%02x%02x%02x\"/>", x*8, y*8, r, g, b)
		}
	}
	_, e = fmt.Fprintln(w, "</svg>")
	return e
}
func mainRun() error {
	var c Config
	var o Options
	flag.StringVar(&c.Mode, "mode", "nebula", "simulation: nebula, vortex, river, constellation, eclipse, swarm")
	flag.IntVar(&c.Width, "width", 90, "simulation width")
	flag.IntVar(&c.Height, "height", 32, "simulation height")
	flag.IntVar(&c.Count, "agents", 180, "initial organisms")
	flag.Int64Var(&c.Seed, "seed", 42, "deterministic random seed")
	flag.Float64Var(&c.Dt, "dt", .65, "simulation time step")
	flag.Float64Var(&c.Coupling, "coupling", 1.1, "phase synchronization coupling")
	flag.Float64Var(&c.Radius, "radius", 9, "interaction radius")
	flag.Float64Var(&c.Trail, "trail", .89, "luminous persistence (0 to 1)")
	flag.Float64Var(&c.Flow, "flow", .18, "environmental force")
	flag.Float64Var(&c.Mutation, "mutation", .2, "offspring mutation amplitude")
	flag.IntVar(&c.Epoch, "epoch", 500, "reproduction interval, 0 disables evolution")
	flag.IntVar(&o.Frames, "frames", 0, "simulation frames, 0 continuous")
	flag.IntVar(&o.FPS, "fps", 20, "maximum display frames per second")
	flag.StringVar(&o.Output, "output", "", "final JSON snapshot path")
	flag.StringVar(&o.Snapshot, "snapshot", "", "alias for -output")
	flag.StringVar(&o.Input, "input", "", "resume from JSON snapshot")
	flag.StringVar(&o.Export, "svg", "", "export final frame as SVG")
	flag.StringVar(&o.CSV, "csv", "", "write per-frame metrics to CSV")
	flag.BoolVar(&o.NoColor, "no-color", false, "disable ANSI truecolor")
	flag.BoolVar(&o.Quiet, "quiet", false, "suppress animation")
	flag.StringVar(&o.Palette, "palette", "ocean", "ocean, ember, aurora, violet, mono")
	flag.StringVar(&o.Render, "render", "ascii", "ascii or braille")
	flag.IntVar(&o.SaveEvery, "save-every", 0, "periodically save snapshot every N frames (requires -output)")
	flag.Parse()
	if flag.NArg() > 0 {
		return errors.New("unexpected positional arguments; use -h for flags")
	}
	modes := map[string]bool{"nebula": true, "vortex": true, "river": true, "constellation": true, "eclipse": true, "swarm": true}
	if !modes[c.Mode] {
		return errors.New("unknown simulation mode")
	}
	if c.Width < 8 || c.Width > 400 || c.Height < 8 || c.Height > 160 || c.Count < 1 || c.Count > 2000 || c.Dt <= 0 || c.Dt > 3 || c.Radius <= 0 || c.Trail < 0 || c.Trail > 1 || c.Epoch < 0 || c.Mutation < 0 || o.FPS < 1 || o.FPS > 120 || o.Frames < 0 || o.SaveEvery < 0 {
		return errors.New("invalid parameter range")
	}
	if o.Render != "ascii" && o.Render != "braille" {
		return errors.New("render must be ascii or braille")
	}
	if !map[string]bool{"ocean": true, "ember": true, "aurora": true, "violet": true, "mono": true}[o.Palette] {
		return errors.New("invalid palette")
	}
	if o.Snapshot != "" {
		o.Output = o.Snapshot
	}
	if o.SaveEvery > 0 && o.Output == "" {
		return errors.New("-save-every requires -output")
	}
	var s *State
	var e error
	if o.Input != "" {
		s, e = load(o.Input)
		if e != nil {
			return e
		}
	} else {
		s = newState(c)
	}
	r := rand.New(rand.NewSource(s.Config.Seed + int64(s.Tick)*7919))
	var cw *csv.Writer
	var cf *os.File
	if o.CSV != "" {
		cf, e = os.Create(o.CSV)
		if e != nil {
			return e
		}
		defer cf.Close()
		cw = csv.NewWriter(cf)
		defer cw.Flush()
		cw.Write([]string{"tick", "time", "population", "phase_order", "mean_energy", "mean_speed", "luminosity", "births", "deaths"})
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)
	if !o.Quiet {
		fmt.Print("\x1b[?25l")
		defer fmt.Print("\x1b[0m\x1b[?25h\n")
	}
	duration := time.Second / time.Duration(o.FPS)
	done := false
	for i := 0; o.Frames == 0 || i < o.Frames; i++ {
		select {
		case <-sig:
			done = true
		default:
		}
		if done {
			break
		}
		start := time.Now()
		step(s, r)
		if cw != nil {
			m := metrics(s)
			cw.Write([]string{strconv.Itoa(m.Tick), fmt.Sprintf("%.4f", m.Time), strconv.Itoa(m.Population), fmt.Sprintf("%.6f", m.Order), fmt.Sprintf("%.6f", m.MeanEnergy), fmt.Sprintf("%.6f", m.MeanSpeed), fmt.Sprintf("%.6f", m.Luminosity), strconv.Itoa(m.Births), strconv.Itoa(m.Deaths)})
		}
		if !o.Quiet {
			fmt.Print("\x1b[H\x1b[2J", render(s, o))
		}
		if o.SaveEvery > 0 && s.Tick%o.SaveEvery == 0 {
			if e = save(o.Output, s); e != nil {
				return e
			}
		}
		if !o.Quiet {
			if delay := duration - time.Since(start); delay > 0 {
				time.Sleep(delay)
			}
		}
	}
	if cw != nil {
		cw.Flush()
		if e = cw.Error(); e != nil {
			return e
		}
	}
	if e = save(o.Output, s); e != nil {
		return e
	}
	if o.Export != "" {
		if e = exportSVG(o.Export, s, o.Palette); e != nil {
			return e
		}
	}
	if o.Quiet {
		m := metrics(s)
		b, _ := json.MarshalIndent(m, "", "  ")
		fmt.Println(string(b))
	}
	return nil
}
func main() {
	if e := mainRun(); e != nil {
		fmt.Fprintln(os.Stderr, "noctiluca:", e)
		os.Exit(1)
	}
}
