// ==============================================================
// Forecast & Predictive Service for Naoja Retail Shop
// Linear Regression + 7-Day Moving Average + Day-of-Week Seasonality
// ==============================================================

/**
 * Calculate linear regression line (y = mx + c)
 */
function linearRegression(points) {
  const n = points.length;
  if (n === 0) return { slope: 0, intercept: 0 };
  if (n === 1) return { slope: 0, intercept: points[0].y };

  let sumX = 0;
  let sumY = 0;
  let sumXY = 0;
  let sumXX = 0;

  for (let i = 0; i < n; i++) {
    const x = points[i].x;
    const y = points[i].y;
    sumX += x;
    sumY += y;
    sumXY += x * y;
    sumXX += x * x;
  }

  const denominator = (n * sumXX - sumX * sumX);
  const slope = denominator !== 0 ? (n * sumXY - sumX * sumY) / denominator : 0;
  const intercept = (sumY - slope * sumX) / n;

  return { slope, intercept };
}

/**
 * Generate forward revenue forecast with confidence intervals
 * @param {Array<{date: string, revenue: number}>} historicalData
 * @param {number} horizonDays - 7, 30, 90, or 365
 */
function generateRevenueForecast(historicalData, horizonDays = 30) {
  if (!historicalData || historicalData.length === 0) {
    return { forecast: [], projectedTotal: 0, growthRate: 0 };
  }

  // Calculate day-of-week seasonality factor (0=Sun, 1=Mon, ..., 6=Sat)
  const dayWeights = [0, 0, 0, 0, 0, 0, 0];
  const dayCounts = [0, 0, 0, 0, 0, 0, 0];

  const totalRev = historicalData.reduce((sum, d) => sum + Number(d.revenue || 0), 0);
  const overallDailyAvg = totalRev / (historicalData.length || 1);

  historicalData.forEach(item => {
    const day = new Date(item.date).getDay();
    dayWeights[day] += Number(item.revenue || 0);
    dayCounts[day] += 1;
  });

  const seasonalityIndices = dayWeights.map((sum, day) => {
    if (dayCounts[day] === 0 || overallDailyAvg === 0) return 1.0;
    const dayAvg = sum / dayCounts[day];
    return Math.max(0.5, Math.min(2.0, dayAvg / overallDailyAvg));
  });

  // Linear trend
  const regressionPoints = historicalData.map((d, index) => ({
    x: index,
    y: Number(d.revenue || 0)
  }));
  const { slope, intercept } = linearRegression(regressionPoints);

  // Moving average (last 7 days)
  const recent7 = historicalData.slice(-7);
  const recent7Avg = recent7.reduce((s, d) => s + Number(d.revenue || 0), 0) / (recent7.length || 1);

  // Project forward
  const forecast = [];
  const lastDate = new Date(historicalData[historicalData.length - 1].date);
  let projectedTotal = 0;

  for (let step = 1; step <= horizonDays; step++) {
    const futureDate = new Date(lastDate);
    futureDate.setDate(lastDate.getDate() + step);
    const dayOfWeek = futureDate.getDay();
    const seasonality = seasonalityIndices[dayOfWeek] || 1.0;

    const trendComponent = intercept + slope * (historicalData.length + step);
    // Blend 60% recent moving average and 40% linear trend
    const basePrediction = Math.max(0, (recent7Avg * 0.6 + trendComponent * 0.4) * seasonality);

    // Confidence interval widens as horizon grows
    const uncertaintyPct = 0.08 + (step / horizonDays) * 0.15;
    const lowerBound = Math.max(0, Math.round(basePrediction * (1 - uncertaintyPct)));
    const upperBound = Math.round(basePrediction * (1 + uncertaintyPct));
    const predictedValue = Math.round(basePrediction);

    projectedTotal += predictedValue;

    forecast.push({
      date: futureDate.toISOString().split('T')[0],
      projected: predictedValue,
      lowerBound,
      upperBound
    });
  }

  const prevPeriodTotal = recent7Avg * horizonDays;
  const growthRate = prevPeriodTotal > 0 ? Number((((projectedTotal - prevPeriodTotal) / prevPeriodTotal) * 100).toFixed(1)) : 0;

  return {
    forecast,
    projectedTotal,
    growthRate
  };
}

/**
 * Generate Product Demand Forecast & Run-out Velocity
 */
function generateDemandForecast(products, salesVelocityMap, horizonDays = 30) {
  return products.map(product => {
    const velocity = salesVelocityMap[product.id] || { daily_7d: 0, daily_30d: 0, daily_90d: 0 };
    // Weighted daily velocity: heavier weight on recent 7 days
    const effectiveDailyVelocity = (velocity.daily_7d * 0.6) + (velocity.daily_30d * 0.3) + (velocity.daily_90d * 0.1);
    const safeVelocity = Math.max(0, effectiveDailyVelocity);

    const currentStock = product.stock_quantity || 0;
    const projectedUnitsDemanded = Math.ceil(safeVelocity * horizonDays);

    let daysRemaining = 999;
    let predictedStockoutDate = null;

    if (safeVelocity > 0) {
      daysRemaining = Number((currentStock / safeVelocity).toFixed(1));
      const targetDate = new Date();
      targetDate.setDate(targetDate.getDate() + Math.floor(daysRemaining));
      predictedStockoutDate = targetDate.toISOString().split('T')[0];
    }

    // Lead time assumption: 3 days in Kakamega/Nairobi supplier
    const leadTimeDays = 3;
    const safetyStock = Math.ceil(safeVelocity * 5); // 5 days safety
    const reorderPoint = Math.ceil(safeVelocity * leadTimeDays) + safetyStock;
    const suggestedReorderQty = Math.max(0, Math.ceil(safeVelocity * 30) - currentStock + safetyStock);

    let status = 'healthy';
    if (currentStock === 0) status = 'out_of_stock';
    else if (currentStock <= safetyStock || daysRemaining <= 5) status = 'critical';
    else if (currentStock <= reorderPoint || daysRemaining <= 14) status = 'low';
    else if (safeVelocity === 0 && currentStock > 0) status = 'dead_stock';

    return {
      productId: product.id,
      name: product.name,
      category_id: product.category_id,
      currentStock,
      dailyVelocity: Number(safeVelocity.toFixed(2)),
      projectedDemand: projectedUnitsDemanded,
      daysRemaining,
      predictedStockoutDate,
      reorderPoint,
      suggestedReorderQty,
      status
    };
  });
}

/**
 * Generate plain-English managerial insights
 */
function generateExecutiveInsights({ summary, revenueTrend, categoryPerformance, inventoryAlerts, paymentHealth, targets }) {
  const insights = [];

  // 1. Revenue & Pace insight
  if (summary && summary.totalRevenue !== undefined) {
    const formattedRev = new Intl.NumberFormat('en-KE', { style: 'currency', currency: 'KES', maximumFractionDigits: 0 }).format(summary.totalRevenue);
    if (summary.revenueGrowth >= 0) {
      insights.push({
        type: 'positive',
        icon: 'trending-up',
        title: 'Strong Revenue Momentum',
        text: `Total period revenue is ${formattedRev}, representing a +${summary.revenueGrowth}% increase compared to the previous timeframe.`
      });
    } else {
      insights.push({
        type: 'warning',
        icon: 'trending-down',
        title: 'Revenue Pace Down',
        text: `Revenue sits at ${formattedRev} (${summary.revenueGrowth}% vs prior period). Consider running promotional bundles or WhatsApp broadcast campaigns.`
      });
    }
  }

  // 2. Category Leader insight
  if (categoryPerformance && categoryPerformance.length > 0) {
    const topCat = categoryPerformance[0];
    insights.push({
      type: 'info',
      icon: 'pie-chart',
      title: `Top Performer: ${topCat.categoryName}`,
      text: `${topCat.categoryName} is your highest contributing department, generating ${topCat.shareOfRevenue}% of total shop revenue (${topCat.unitsSold} units sold).`
    });
  }

  // 3. Stock Out Risk insight
  const criticalItems = (inventoryAlerts || []).filter(a => a.status === 'critical' || a.status === 'out_of_stock');
  if (criticalItems.length > 0) {
    const itemNames = criticalItems.slice(0, 2).map(i => i.name).join(' & ');
    insights.push({
      type: 'critical',
      icon: 'alert-triangle',
      title: `${criticalItems.length} Products Need Reordering Urgently`,
      text: `Key inventory including "${itemNames}" are nearing complete stock-out within 3–5 days based on recent daily velocity.`
    });
  } else {
    insights.push({
      type: 'positive',
      icon: 'check-circle',
      title: 'Healthy Warehouse Stock Levels',
      text: `No critical stock bottlenecks detected. All fast-moving inventory has more than 14 days of coverage.`
    });
  }

  // 4. Payment Conversion insight
  if (paymentHealth) {
    const successRate = paymentHealth.successRate || 95;
    if (successRate >= 90) {
      insights.push({
        type: 'positive',
        icon: 'credit-card',
        title: 'High M-Pesa Payment Conversion',
        text: `Lipa Na M-Pesa success rate is currently ${successRate}%. Transactions are completing smoothly without Daraja gateway timeout spikes.`
      });
    } else {
      insights.push({
        type: 'warning',
        icon: 'alert-circle',
        title: 'M-Pesa Drop-off Spike',
        text: `Payment completion rate is ${successRate}%. Common reason: user cancelled prompt or insufficient Safaricom M-Pesa balance.`
      });
    }
  }

  // 5. Target progress insight
  if (targets && targets.targetProgress !== undefined) {
    const status = targets.targetProgress >= 100 ? 'surpassed' : targets.targetProgress >= 70 ? 'on track' : 'lagging behind';
    insights.push({
      type: targets.targetProgress >= 70 ? 'positive' : 'warning',
      icon: 'target',
      title: `Periodic Target Status: ${status.toUpperCase()}`,
      text: `You have achieved ${targets.targetProgress}% of your KES ${Number(targets.revenueTarget || 0).toLocaleString()} target for this cycle.`
    });
  }

  return insights;
}

module.exports = {
  linearRegression,
  generateRevenueForecast,
  generateDemandForecast,
  generateExecutiveInsights
};
