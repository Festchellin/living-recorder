import { useEffect, useRef, useState } from 'react'
import mpegts from 'mpegts.js'

export interface PreviewConfig {
  width: number
  height: number
  fps: number
  crf: number
}

export function useMpegtsPreview(streamId: number | null, config?: PreviewConfig) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const playerRef = useRef<mpegts.Player | null>(null)
  const [status, setStatus] = useState('')

  useEffect(() => {
    if (!streamId) return
    let cancelled = false

    setStatus('连接中...')

    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    let wsUrl = `${proto}//${location.host}/api/preview/${streamId}/ws`
    if (config) {
      wsUrl += `?width=${config.width}&height=${config.height}&fps=${config.fps}&crf=${config.crf}`
    }

    if (mpegts.isSupported()) {
      const player = mpegts.createPlayer(
        { type: 'mpegts', isLive: true, url: wsUrl },
        {
          enableWorker: false,
          lazyLoad: false,
          liveBufferLatencyChasing: true,
          fixAudioTimestampGap: true,
        },
      )
      playerRef.current = player
      player.attachMediaElement(videoRef.current!)
      player.load()
      player.play()
      setStatus('已连接')

      player.on(mpegts.Events.ERROR, () => {
        if (!cancelled) setStatus('连接失败')
      })
    } else {
      setStatus('浏览器不支持 MSE')
    }

    return () => {
      cancelled = true
      if (playerRef.current) {
        playerRef.current.destroy()
        playerRef.current = null
      }
    }
  }, [streamId, config])

  return { videoRef, status }
}
