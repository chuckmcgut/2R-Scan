package com.ripscan.grading

import android.graphics.Bitmap
import kotlin.math.abs
import kotlin.math.max
import kotlin.math.min

data class GradeBreakdown(
    val centering: Double = 0.0,
    val corners: Double = 0.0,
    val edges: Double = 0.0,
    val surface: Double = 0.0
)

data class GradeResult(
    val overall: Double,
    val breakdown: GradeBreakdown,
    val confidence: Double,
    val cardName: String? = null
) {
    fun toDisplayString() = "PSA %.1f (C:%.1f | Co:%.1f | E:%.1f | S:%.1f)".format(
        overall, breakdown.centering, breakdown.corners, breakdown.edges, breakdown.surface)
}

object GradingEngine {
    fun grade(bitmap: Bitmap): GradeResult {
        val w = bitmap.width
        val h = bitmap.height
        val centering = analyzeCentering(bitmap, w, h)
        val corners = analyzeCorners(bitmap, w, h)
        val edges = analyzeEdges(bitmap, w, h)
        val surface = analyzeSurface(bitmap, w, h)
        val overall = (centering * 0.20 + corners * 0.20 + edges * 0.25 + surface * 0.35) * 10
        val rounded = (overall * 10).toInt() / 10.0
        val confidence = calculateConfidence(centering, corners, edges, surface)
        return GradeResult(rounded, GradeBreakdown(centering, corners, edges, surface), confidence)
    }

    private fun analyzeCentering(bmp: Bitmap, w: Int, h: Int): Double {
        val top = findTopEdge(bmp, w, h)
        val bottom = findBottomEdge(bmp, w, h)
        val left = findLeftEdge(bmp, w, h)
        val right = findRightEdge(bmp, w, h)
        if (top == 0 || bottom == 0 || left == 0 || right == 0) return 0.5
        val topDev = abs(top - bottom).toDouble() / h
        val leftDev = abs(left - right).toDouble() / w
        val maxDev = max(topDev, leftDev)
        return (1.0 - (maxDev / 0.10)).coerceIn(0.0, 1.0)
    }

    private fun findTopEdge(bmp: Bitmap, w: Int, h: Int): Int {
        for (y in 0 until h / 2) {
            for (x in 0 until w) {
                if (isSignificantPixel(bmp.getPixel(x, y))) return y
            }
        }
        return 0
    }

    private fun findBottomEdge(bmp: Bitmap, w: Int, h: Int): Int {
        for (y in h - 1 downTo h / 2) {
            for (x in 0 until w) {
                if (isSignificantPixel(bmp.getPixel(x, y))) return y
            }
        }
        return 0
    }

    private fun findLeftEdge(bmp: Bitmap, w: Int, h: Int): Int {
        for (x in 0 until w / 2) {
            for (y in 0 until h) {
                if (isSignificantPixel(bmp.getPixel(x, y))) return x
            }
        }
        return 0
    }

    private fun findRightEdge(bmp: Bitmap, w: Int, h: Int): Int {
        for (x in w - 1 downTo w / 2) {
            for (y in 0 until h) {
                if (isSignificantPixel(bmp.getPixel(x, y))) return x
            }
        }
        return 0
    }

    private fun isSignificantPixel(pixel: Int): Boolean {
        val r = (pixel shr 16) and 0xFF
        val g = (pixel shr 8) and 0xFF
        val b = pixel and 0xFF
        val avg = (r + g + b) / 3
        val contrast = abs(r - avg) + abs(g - avg) + abs(b - avg)
        return contrast > 15 && (pixel shr 24) and 0xFF != 0
    }

    private fun analyzeCorners(bmp: Bitmap, w: Int, h: Int): Double {
        val cornerSize = w / 6
        val corners = arrayOf(intArrayOf(0, 0), intArrayOf(w - cornerSize, 0),
            intArrayOf(0, h - cornerSize), intArrayOf(w - cornerSize, h - cornerSize))
        var totalDamage = 0.0
        var samples = 0
        for (corner in corners) {
            val cx = corner[0]; val cy = corner[1]
            for (y in cy until min(cy + cornerSize, h)) {
                for (x in cx until min(cx + cornerSize, w)) {
                    val px = bmp.getPixel(x, y)
                    val alpha = (px shr 24) and 0xFF
                    if (alpha < 128) continue
                    val r = (px shr 16) and 0xFF; val g = (px shr 8) and 0xFF; val b = px and 0xFF
                    val avg = (r + g + b) / 3
                    if (avg > 200) totalDamage += 1.0
                    val contrast = abs(r - avg) + abs(g - avg) + abs(b - avg)
                    if (contrast > 40) totalDamage += 1.0
                    samples++
                }
            }
        }
        if (samples == 0) return 0.5
        return (1.0 - (totalDamage / samples) * 3).coerceIn(0.0, 1.0)
    }

    private fun analyzeEdges(bmp: Bitmap, w: Int, h: Int): Double {
        val edgeWidth = w / 20
        val regions = arrayOf(
            intArrayOf(0, 0, w, edgeWidth),
            intArrayOf(0, h - edgeWidth, w, edgeWidth),
            intArrayOf(0, 0, edgeWidth, h),
            intArrayOf(w - edgeWidth, 0, edgeWidth, h)
        )
        var totalDamage = 0.0
        var samples = 0
        for (region in regions) {
            val rx = region[0]; val ry = region[1]; val rw = region[2]; val rh = region[3]
            for (y in ry until min(ry + rh, h)) {
                for (x in rx until min(rx + rw, w)) {
                    val px = bmp.getPixel(x, y)
                    val alpha = (px shr 24) and 0xFF
                    if (alpha < 128) continue
                    val r = (px shr 16) and 0xFF; val g = (px shr 8) and 0xFF; val b = px and 0xFF
                    val avg = (r + g + b) / 3
                    if (avg > 210) totalDamage += 1.0
                    val contrast = abs(r - avg) + abs(g - avg) + abs(b - avg)
                    if (contrast > 35) totalDamage += 1.0
                    samples++
                }
            }
        }
        if (samples == 0) return 0.5
        return (1.0 - (totalDamage / samples) * 4).coerceIn(0.0, 1.0)
    }

    private fun analyzeSurface(bmp: Bitmap, w: Int, h: Int): Double {
        val startX = w / 2 - w / 6
        val startY = h / 2 - h / 6
        var totalDamage = 0.0
        var samples = 0
        for (y in startY until min(startY + w / 3, h)) {
            for (x in startX until min(startX + w / 3, w)) {
                if (x <= 0) continue
                val px = bmp.getPixel(x, y)
                val alpha = (px shr 24) and 0xFF
                if (alpha < 128) continue
                val r = (px shr 16) and 0xFF; val g = (px shr 8) and 0xFF; val b = px and 0xFF
                val avg = (r + g + b) / 3
                if (x > 0) {
                    val lp = bmp.getPixel(x - 1, y)
                    val lr = (lp shr 16) and 0xFF; val lg = (lp shr 8) and 0xFF; val lb = lp and 0xFF
                    val lavg = (lr + lg + lb) / 3
                    if (abs(avg - lavg) > 30) totalDamage += 1.0
                }
                if (avg < 50) totalDamage += 1.0
                samples++
            }
        }
        if (samples == 0) return 0.5
        return (1.0 - (totalDamage / samples) * 5).coerceIn(0.0, 1.0)
    }

    private fun calculateConfidence(c: Double, co: Double, e: Double, s: Double): Double {
        val avg = (c + co + e + s) / 4
        val spread = abs(c - avg) + abs(co - avg) + abs(e - avg) + abs(s - avg)
        return (avg * (1.0 - spread / 2)).coerceIn(0.5, 1.0)
    }
}