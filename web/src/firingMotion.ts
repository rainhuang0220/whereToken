import gsap from 'gsap'

export function tweenCharge(el: HTMLElement, amount: number, reduced: boolean) {
  el.style.willChange = 'transform'
  gsap.to(el, {
    scaleX: amount,
    duration: reduced ? 0 : 0.4,
    ease: 'power2.out',
    transformOrigin: 'left center',
    overwrite: 'auto',
  })
}

export function tweenVeil(el: HTMLElement, show: boolean, reduced: boolean) {
  gsap.fromTo(
    el,
    { autoAlpha: show ? 0 : 1 },
    {
      autoAlpha: show ? 1 : 0,
      duration: reduced ? 0 : 0.22,
      ease: 'power1.out',
      overwrite: 'auto',
    },
  )
}
