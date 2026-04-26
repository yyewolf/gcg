# Algorithms

This document explains the main algorithms used by the current Galcon prototype.

The most important code paths live in these files:

- `internal/game/engine.go`
- `internal/game/avoidance.go`
- `internal/game/collision.go`
- `internal/game/types.go`
- `internal/server/hub.go`
- `web/src/app/screens/game/scene/GameBoard.ts`
- `web/src/app/screens/game/scene/FleetView.ts`
- `web/src/app/screens/game/scene/PlanetView.ts`

## 1. Simulation Model

The server is authoritative.

Planets and fleets live only in memory. The server advances the simulation at a fixed tick rate and broadcasts snapshots over WebSocket.

At a high level each tick does this:

1. Increment the global tick.
2. Move all fleets.
3. Resolve fleet-fleet collisions.
4. Recompute ETA for active fleets.
5. Grow owned planets once per second.
6. Emit a sorted snapshot.

The relevant entrypoint is `Engine.Advance()`.

## 2. World State

### Planets

Each planet stores:

- position `(x, y)`
- radius
- owner
- current ship count
- growth rate

### Fleets

Each fleet stores:

- identity and ownership
- source and target planet ids
- a ship count
- launch tick and estimated arrival tick
- position `(x, y)`
- velocity `(vx, vy)`
- avoidance memory for the currently selected blocking planet

Important detail: the current backend uses one simulated fleet entity per ship for launched groups.

That means launching `40` ships creates `40` fleet entities with `Ships = 1` each.

## 3. Fleet Launch Algorithm

When the player sends ships:

1. Validate the order.
2. Compute `shipsToSend = source.Ships * percentage / 100`.
3. Subtract ships from the source planet.
4. Compute the straight-line travel vector toward the target.
5. Spawn one backend fleet per ship.

### Ring-Based Spawn

Ships are  spawned on concentric rings around the source planet:

1. Compute the launch direction from source to target.
2. Start at a base launch radius:

$$
r_0 = source.radius + fleetCollisionRadius + collisionPadding
$$

3. Fill a ring with as many ships as its circumference can hold:

$$
capacity(r) = \left\lfloor \frac{2\pi r}{shipLaunchSpacing} \right\rfloor
$$

4. If ships remain, create another ring at:

$$
r_{n+1} = r_n + shipLaunchSpacing
$$

5. Distribute ships evenly by angle on each ring.

The first slot is aligned with the target direction so the formation still has a forward bias.

### Why This Helps

The older line-based spawn caused large launches to explode sideways immediately because every ship started offset on the same axis and then had to resolve local collisions.

The ring-based spawn spreads ships radially around the source perimeter and avoids that initial singular line.

## 4. Movement Model

The current movement model is not a pure physics integrator and not a waypoint-following state machine.

It is a hybrid steering model with these properties:

- constant speed magnitude
- bounded turning rate
- per-tick recomputed desired heading
- local collision avoidance
- path-side selection around blocking planets based on shortest tangent route

### Step A: Compute Desired Direction

For each fleet, the engine computes a desired direction vector each tick.

That vector starts with attraction toward the target:

$$
\vec{d}_{target} = normalize(target - position)
$$

Then it adds steering influences from:

- nearby planets
- the currently selected blocking planet
- nearby fleets

The function responsible for this is `computeFleetAccelerationLocked()`.

The name still says "acceleration", but in the current implementation the result is better thought of as a desired steering vector, not true accumulated acceleration.

### Step B: Turn Toward Desired Direction

Ships do not instantly snap to the desired heading.

Instead, they rotate their current heading toward the desired heading by at most:

$$
maxTurn = fleetTurnRateRadians \cdot \Delta t
$$

This is implemented by:

1. converting the current and desired vectors to angles
2. computing the shortest signed angular difference
3. clamping that difference to the max turn rate
4. reconstructing the next heading from the clamped angle

Then the ship velocity is set to:

$$
\vec{v} = heading \cdot fleetSpeed
$$

So ships keep constant top speed but can only reorient at a limited rate.

### Step C: Advance Position

With fixed `vx` and `vy`, position updates are simple:

$$
x' = x + vx \cdot \Delta t
$$

$$
y' = y + vy \cdot \Delta t
$$

Before accepting the move, the engine checks whether the segment from current to next position intersects the target planet’s arrival circle.

If it does, the fleet resolves immediately as an arrival.

## 5. Planet Avoidance Algorithm

The system does not let ships blindly orbit every nearby planet.

Instead, it identifies a single blocking planet for path-side selection, and uses lighter repulsion for all other nearby planets.

### 5.1 Detecting the Blocking Planet

The engine scans planets and selects the nearest one that intersects the straight segment from the ship to the target, excluding:

- the source planet
- the target planet

The geometric test is segment-vs-circle intersection.

This yields the planet most immediately blocking the current direct route.

### 5.2 Keeping or Releasing the Current Obstacle

Once a fleet has committed to a blocking planet, it keeps that planet while either of these is true:

- the direct line to the target still intersects that planet’s clearance circle
- the fleet is still within the planet influence band

That prevents per-tick obstacle churn.

### 5.3 Choosing Clockwise vs Counterclockwise

This is where the system becomes path-oriented rather than purely local.

For the chosen blocking planet, the engine evaluates both tangent routes:

- clockwise
- counterclockwise

For each side it computes:

1. tangent point from current fleet position to the clearance circle
2. tangent point from target position to the same clearance circle
3. path length as:

$$
approach + arc + exit
$$

where:

$$
approach = distance(current, entryTangent)
$$

$$
arc = angularDistance(entryAngle, exitAngle) \cdot clearanceRadius
$$

$$
exit = distance(exitTangent, target)
$$

The shorter of the two routes wins.

There is also a stability margin (`avoidanceSwitchMargin`) so a fleet that is already committed to one side does not switch for tiny improvements.

### 5.4 Applying Planet Steering

For nearby planets, the desired vector gets an outward push away from the planet center.

For the selected blocking planet, the desired vector also gets a tangential component on the chosen side.

This produces behavior that still looks smooth in motion, but the chosen side itself is selected by a shortest-route test rather than by a local sign check.

## 6. Planet Collision Response

After moving, fleets are clamped against all non-target planets.

If a fleet ends inside a planet clearance radius:

1. project its position back to the clearance circle
2. compute the inward component of velocity along the surface normal
3. remove only that inward component

So the ship stops moving into the planet but may continue tangentially along the surface.

This creates the "slide around the planet" behavior instead of a bounce.

## 7. Fleet-Fleet Separation and Collision

Ships also avoid and collide with one another.

There are two layers:

### Steering Separation

During desired-direction computation, nearby fleets contribute a repulsion term.

This keeps dense swarms from collapsing into the same point.

### Post-Move Collision Resolution

After all fleets move, the engine resolves actual overlaps.

For each nearby pair closer than `fleetSeparationDistance`:

1. compute overlap
2. move both ships apart by half the overlap
3. compute relative velocity along the contact normal
4. if they are moving into each other, apply a damped impulse

This is not a full rigid-body solver. It is a light pairwise separation step designed to be cheap and visually stable.

## 8. Spatial Acceleration Structure

The main optimization for large fleet counts is a uniform spatial grid.

Without it, fleet-fleet steering and collision checks are `O(n^2)`.

At `800` ships, that becomes expensive quickly.

### Grid Structure

The engine builds a `fleetSpatialIndex`:

- choose a `cellSize`
- hash each fleet into integer cell coordinates
- store fleets in `map[cell][]*Fleet`

Two separate grids are built each tick:

1. a steering grid with cell size `fleetSeparationDistance + fleetInfluencePadding`
2. a collision grid with cell size `fleetSeparationDistance`

### Neighbor Query

To find nearby fleets for a point `(x, y)` and radius `r`:

1. compute the min/max grid cells overlapped by the query circle
2. iterate only those cells
3. visit only fleets stored in those cells

This reduces neighbor work from global all-vs-all scans to local neighborhood scans.

In practice, this is the main reason the current model scales substantially better than the older naive loops.

## 9. Arrival and Combat Resolution

If a fleet segment intersects the target planet arrival circle during movement, arrival resolves immediately.

The combat rule is simple:

- if the target owner matches the fleet owner, add ships
- otherwise subtract ships
- if the result goes negative, ownership flips and the absolute remainder becomes the new ship count

Because the backend currently launches one fleet entity per ship, arrivals happen as many small repeated updates rather than one large lump.

That is useful for visuals and fine-grained behavior, but it also increases total entity count and simulation load.

## 10. ETA Estimation

ETA is not a guaranteed arrival contract.

It is a rolling estimate recomputed every tick from straight-line distance to the target edge:

$$
remaining = distance(current, targetCenter) - (targetRadius + fleetCollisionRadius)
$$

Then:

$$
ETA = currentTick + \left\lceil \frac{remaining / fleetSpeed}{tickDuration} \right\rceil
$$

Because avoidance and collisions can change the actual path, ETA is only approximate.

## 11. Snapshot Production

Before sending state to clients, planets and fleets are copied into slices sorted by id.

This gives deterministic ordering for the frontend and reduces visual churn from map iteration randomness.

## 12. Frontend Rendering Algorithms

The frontend is mostly presentational, but a few algorithms matter.

### Fleet Rendering

`FleetView` renders each backend fleet as a small ship glyph.

The glyph rotates from the velocity vector:

$$
rotation = atan2(vy, vx)
$$

Debug path trails exist but are disabled by default.

### Planet Impact Effects

`GameBoard` tracks the previous snapshot.

When a fleet present in the previous snapshot disappears from the new snapshot, the board interprets that as a landing event.

The effect only triggers when the previous target owner was not the local player.

That means:

- hostile and neutral landings can create impacts
- reinforcing your own planets does not

`PlanetView` then runs a short fire-style animation:

- expanding glow
- hot orange/yellow core
- white hot center
- upward sparks

This is a frontend-only effect. The backend does not know about explosions.

## 13. Complexity Summary

For `P` planets and `F` fleets:

- planet growth: `O(P)` once per second
- blocking-planet scan per fleet: `O(P)`
- steering neighbor scan per fleet: roughly local, not global, due to the grid
- collision neighbor scan per fleet: roughly local, not global, due to the grid
- snapshot serialization: `O(P log P + F log F)` because ids are sorted

Given the current map size, planets are few and cheap. Fleet count is the dominant cost.

The main expensive pieces today are:

- one entity per ship
- per-fleet path-side selection around planets
- per-tick rebuilding of the spatial index
- sorting fleets for snapshots

## 14. Current Tradeoffs

The current design deliberately favors clarity and gameplay experimentation over maximum throughput.

### Advantages

- simple authoritative server model
- visually understandable movement rules
- per-ship granularity enables dense swarm behavior
- path side selection is more robust than purely local steering
- spatial grid avoids the worst `O(n^2)` behavior for fleet interactions

### Costs

- one ship per entity increases memory, update cost, and snapshot size
- ETA is approximate, not exact
- path selection still reasons about one blocking planet at a time rather than solving a global multi-obstacle shortest path
- snapshot broadcasts can become heavy at large fleet counts

## 15. If You Want to Evolve the Algorithms Further

The next likely algorithmic upgrades would be:

1. fleet batching for networking while keeping per-ship backend simulation
2. multi-obstacle lookahead instead of single-blocking-planet routing
3. persistent spatial index reuse between steering and collision passes
4. snapshot delta compression
5. parallel simulation or lock-free staging if entity counts continue rising

For now, the current system is a good compromise between readable code, convincing motion, and moderate-scale performance.