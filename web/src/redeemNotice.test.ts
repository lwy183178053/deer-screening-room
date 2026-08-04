import { describe, expect, it } from 'vitest'
import { splitRedeemNoticeLinks } from './redeemNotice'

describe('redeem notice links', () => {
  it('keeps plain text unchanged', () => {
    expect(splitRedeemNoticeLinks('请先购买卡密，再回到这里兑换。')).toEqual([
      { text: '请先购买卡密，再回到这里兑换。' },
    ])
  })

  it('extracts multiple HTTP links while preserving line breaks and punctuation', () => {
    expect(splitRedeemNoticeLinks('购买地址：\nhttps://shop.example.com/cards，备用 http://backup.example.com/list。')).toEqual([
      { text: '购买地址：\n' },
      { text: 'https://shop.example.com/cards', href: 'https://shop.example.com/cards' },
      { text: '，备用 ' },
      { text: 'http://backup.example.com/list', href: 'http://backup.example.com/list' },
      { text: '。' },
    ])
  })

  it('does not link unsupported or incomplete URLs', () => {
    expect(splitRedeemNoticeLinks('javascript:alert(1) https://')).toEqual([
      { text: 'javascript:alert(1) https://' },
    ])
  })
})
