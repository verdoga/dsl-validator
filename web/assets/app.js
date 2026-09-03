"use strict";
const progress=document.querySelector("[data-progress]");
if(progress){const poll=async()=>{const response=await fetch("/api/progress",{headers:{Accept:"application/json"}});if(response.ok){const state=await response.json();progress.textContent=`Обработано ${state.processed} из ${state.total}`;document.querySelector("[role=status]").textContent=state.event;if(!state.finished)setTimeout(poll,700)}};poll()}
