'use client'

import { useRouter } from 'next/navigation'

/**
 * a real <button> that navigates.
 * ! the stylesheet scopes several rules to the element rather than the class —
 * ! `.post__who .nm button`, `.main__hd .crumb button`, `.presence button`, `.seg button`,
 * ! `.prof .stats button`. a next <Link> renders an <a>, so in those places it picks up none
 * ! of the design's styling and lands as underlined default-blue text instead
 * ? so anywhere the mockup used a button to navigate, this is used rather than a Link. the
 * ? places the mockup used a real link keep Link, which is also what keeps middle-click and
 * ? open-in-new-tab working wherever that was the design's own choice
 */
export function NavButton({
  href,
  className,
  title,
  children,
  ...rest
}: {
  href: string
  className?: string
  title?: string
  children: React.ReactNode
} & Omit<React.ButtonHTMLAttributes<HTMLButtonElement>, 'onClick'>) {
  const router = useRouter()
  return (
    <button type="button" className={className} title={title} onClick={() => router.push(href)} {...rest}>
      {children}
    </button>
  )
}
