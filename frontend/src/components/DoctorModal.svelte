<script>
import { createEventDispatcher, onMount } from "svelte";
import { CheckCircle2, CircleAlert, LoaderCircle, Stethoscope, Wrench, X } from "@lucide/svelte";
import { api } from "../shared/api";
import { createDoctorState, doctorRows, doctorStatusLabel, mergeDoctorPatch } from "../lib/doctorUi";

export let settings = {};

const dispatch = createEventDispatcher();
let rows = createDoctorState();
let localSettings = { ...settings };
let generation = 0;
let repairing = "";

const updateRow = (id, patch) => {
	rows = {
		...rows,
		[id]: { ...rows[id], ...patch },
	};
};

const checkOne = async (id, token = generation) => {
	updateRow(id, {
		status: "checking",
		message: "Проверяем…",
		repairable: false,
	});
	try {
		const result = await api.doctorCheck(id, localSettings);
		if (token !== generation) return;
		updateRow(id, {
			...result,
			status: result?.status || "blocked",
			message: result?.message || "Проверка не вернула результат",
		});
	} catch (error) {
		if (token !== generation) return;
		updateRow(id, {
			status: "blocked",
			message: error?.message || String(error),
			repairable: false,
		});
	}
};

const runChecks = async () => {
	const token = ++generation;
	rows = createDoctorState();
	for (const row of doctorRows) {
		if (token !== generation) return;
		await checkOne(row.id, token);
	}
};

const repair = async (id) => {
	if (repairing) return;
	const token = generation;
	repairing = id;
	updateRow(id, {
		status: "checking",
		message: "Исправляем…",
		repairable: false,
	});
	try {
		const result = await api.doctorRepair(id, localSettings);
		if (token !== generation) return;
		if (result?.patch) {
			localSettings = mergeDoctorPatch(localSettings, result.patch);
			dispatch("patch", result.patch);
		}
		if (result?.check) {
			updateRow(id, result.check);
		} else {
			await checkOne(id);
		}
		if (id === "onnxruntime" && rows[id]?.status === "ready") {
			await checkOne("minilm");
			await checkOne("essentia");
		}
	} catch (error) {
		updateRow(id, {
			status: "repairable",
			message: error?.message || String(error),
			repairable: true,
		});
	} finally {
		repairing = "";
	}
};

const close = () => {
	generation += 1;
	dispatch("close");
};

onMount(() => {
	void runChecks();
	return () => {
		generation += 1;
	};
});
</script>

<div
	class="doctor-overlay"
	role="presentation"
	on:pointerdown={(event) => event.target === event.currentTarget && close()}
>
	<div
		class="doctor-modal"
		role="dialog"
		aria-modal="true"
		aria-labelledby="doctor-title"
		tabindex="-1"
	>
		<header class="doctor-header">
			<div class="doctor-title-wrap">
				<div class="doctor-icon"><Stethoscope size={19} /></div>
				<div>
					<h2 id="doctor-title">Доктор</h2>
					<p>Проверка среды, моделей и вспомогательных инструментов.</p>
				</div>
			</div>
			<div class="doctor-header-actions">
				<button type="button" class="doctor-refresh" on:click={runChecks} disabled={Boolean(repairing)}>Проверить снова</button>
				<button type="button" class="doctor-close" aria-label="Закрыть Доктор" on:click={close}><X size={18} /></button>
			</div>
		</header>

		<div class="doctor-list">
			{#each doctorRows as definition}
				{@const row = rows[definition.id]}
				<div class:ready={row.status === "ready"} class:repairable={row.status === "repairable"} class:blocked={row.status === "blocked"} class:testing={row.status === "pending" || row.status === "checking"} class="doctor-row">
					<div class="doctor-row-mark" aria-hidden="true">
						{#if row.status === "ready"}
							<CheckCircle2 size={18} />
						{:else if row.status === "blocked"}
							<CircleAlert size={18} />
						{:else if row.status === "repairable"}
							<Wrench size={17} />
						{:else}
							<span class="doctor-spinner"><LoaderCircle size={18} /></span>
						{/if}
					</div>
					<div class="doctor-row-copy">
						<strong>{row.title || definition.title}</strong>
						<span>{row.message}</span>
						{#if row.path}<small title={row.path}>{row.path}</small>{/if}
					</div>
					<div class="doctor-row-action">
						{#if row.status === "pending" || row.status === "checking"}
							<div class="doctor-status testing-status"><span class="doctor-spinner"><LoaderCircle size={15} /></span> Проверка</div>
					{:else if row.status === "repairable" && row.repairable}
							<button type="button" class="doctor-fix" on:click={() => repair(definition.id)} disabled={Boolean(repairing)}>
								{repairing === definition.id ? "Исправляем…" : doctorStatusLabel(row)}
							</button>
					{:else}
							<div class="doctor-status">{doctorStatusLabel(row)}</div>
					{/if}
					</div>
				</div>
			{/each}
		</div>

		<footer class="doctor-footer">
			<span>Автоисправление загружает зависимости только в папку данных Ray Player и не требует прав администратора.</span>
		</footer>
	</div>
</div>

<style>
	.doctor-overlay {
		position: fixed;
		inset: 0;
		z-index: 1120;
		display: grid;
		place-items: center;
		padding: 18px;
		background: rgba(4, 7, 12, 0.46);
		backdrop-filter: blur(9px);
		-webkit-backdrop-filter: blur(9px);
		contain: paint;
	}

	.doctor-modal {
		width: min(720px, 100%);
		max-height: min(720px, calc(100dvh - 36px));
		overflow: hidden;
		border: 1px solid rgba(255, 255, 255, 0.10);
		border-radius: 22px;
		background: rgba(17, 20, 27, 0.985);
		box-shadow: 0 30px 90px rgba(0, 0, 0, 0.52);
		color: var(--text);
	}

	.doctor-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 18px;
		padding: 18px 18px 14px;
		border-bottom: 1px solid rgba(255, 255, 255, 0.07);
	}

	.doctor-title-wrap,
	.doctor-header-actions {
		display: flex;
		align-items: center;
		gap: 11px;
	}

	.doctor-icon {
		display: grid;
		width: 38px;
		height: 38px;
		place-items: center;
		border-radius: 12px;
		background: rgba(255, 255, 255, 0.07);
		color: rgba(238, 244, 255, 0.94);
	}

	h2 { margin: 0; font-size: 18px; font-weight: 680; letter-spacing: -0.02em; }
	p { margin: 3px 0 0; color: var(--muted); font-size: 12px; }

	.doctor-refresh,
	.doctor-close,
	.doctor-fix {
		border: 0;
		font: inherit;
		color: inherit;
		cursor: pointer;
	}
	.doctor-refresh { padding: 7px 10px; border-radius: 9px; background: rgba(255,255,255,.06); font-size: 12px; }
	.doctor-close { display: grid; width: 32px; height: 32px; place-items: center; border-radius: 10px; background: rgba(255,255,255,.055); }
	button:disabled { opacity: .48; cursor: default; }

	.doctor-list { display: grid; gap: 1px; padding: 12px; overflow: auto; }
	.doctor-row {
		display: grid;
		grid-template-columns: 28px minmax(0, 1fr) auto;
		align-items: center;
		gap: 10px;
		min-height: 66px;
		padding: 10px 12px;
		border: 1px solid transparent;
		border-radius: 13px;
		background: rgba(255, 255, 255, 0.035);
	}
	.doctor-row.ready { border-color: rgba(48, 209, 88, .18); background: rgba(48, 209, 88, .055); }
	.doctor-row.repairable { border-color: rgba(255, 189, 46, .20); background: rgba(255, 189, 46, .06); }
	.doctor-row.blocked { border-color: rgba(255, 69, 58, .20); background: rgba(255, 69, 58, .06); }
	.doctor-row.testing { border-color: rgba(142, 142, 147, .13); }
	.doctor-row-mark { display: grid; place-items: center; color: #8e8e93; }
	.ready .doctor-row-mark { color: #30d158; }
	.repairable .doctor-row-mark { color: #ffd60a; }
	.blocked .doctor-row-mark { color: #ff453a; }
	.doctor-row-copy { min-width: 0; display: grid; gap: 3px; }
	.doctor-row-copy strong { font-size: 13px; font-weight: 620; }
	.doctor-row-copy span { color: rgba(226,231,240,.68); font-size: 12px; line-height: 1.35; }
	.doctor-row-copy small { overflow: hidden; color: rgba(174,181,194,.48); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
	.doctor-row-action { min-width: 86px; display: flex; justify-content: flex-end; }
	.doctor-status { color: rgba(224,229,238,.62); font-size: 12px; white-space: nowrap; }
	.ready .doctor-status { color: #30d158; }
	.blocked .doctor-status { color: #ff6961; }
	.testing-status { display: flex; align-items: center; gap: 6px; color: #8e8e93; }
	.doctor-fix { padding: 7px 11px; border-radius: 9px; background: rgba(255, 214, 10, .14); color: #ffd60a; font-size: 12px; font-weight: 620; }
	.doctor-spinner { display: inline-flex; animation: doctor-spin .9s linear infinite; }
	.doctor-footer { padding: 10px 18px 15px; color: rgba(184,190,202,.52); font-size: 10px; line-height: 1.4; border-top: 1px solid rgba(255,255,255,.05); }
	@keyframes doctor-spin { to { transform: rotate(360deg); } }
	@media (prefers-reduced-motion: reduce) { .doctor-spinner { animation-duration: 1.8s; } }
	@media (max-width: 620px) {
		.doctor-header { align-items: flex-start; }
		.doctor-refresh { display: none; }
		.doctor-row { grid-template-columns: 25px minmax(0, 1fr); }
		.doctor-row-action { grid-column: 2; justify-content: flex-start; }
	}
</style>
