namespace DahuoChat;

public static class DemoSuggestions
{
    public static IReadOnlyList<ReplyItem> Pick(SuggestAdjust adjust) => adjust switch
    {
        SuggestAdjust.Polish => Polish(),
        SuggestAdjust.Shorter => Shorter(),
        SuggestAdjust.Warmer => Warmer(),
        _ => Wechat()
    };

    public static List<ReplyItem> Wechat() =>
    [
        new() { Tag = "温暖体贴", Text = "太替你高兴了！可惜周六要陪孩子去医院复查，下周我单独上门给你暖房～", Why = "先接住他的喜悦，再给真实原因和补偿" },
        new() { Tag = "幽默化解", Text = "你下厨我必须到场品鉴！可周六得陪娃复查😭 下周我带酒来补作业", Why = "玩笑冲淡拒绝，他不会觉得被冷落" },
        new() { Tag = "委婉拒绝", Text = "这么重要的日子真不想错过，周六得陪孩子复查，替我跟老王他们说声抱歉", Why = "先表重视再说原因，拒绝不伤人" },
        new() { Tag = "真诚夸赞", Text = "看照片就知道装得特别用心！周六实在走不开，下周一定来参观学习", Why = "夸具体的用心，他的辛苦被看见" },
        new() { Tag = "轻松简短", Text = "恭喜乔迁！周六陪娃复查去不了，下周请你吃饭补上", Why = "跟他的轻快语气一致，不拖沓" }
    ];

    public static List<ReplyItem> Warmer() =>
    [
        new() { Tag = "温暖体贴", Text = "太替你高兴了，新家一定很暖。周六要陪孩子复查，下周我一定上门给你暖房。", Why = "把高兴说得更满，再把原因和补偿说清" },
        new() { Tag = "真诚夸赞", Text = "你亲自下厨我都觉得好幸福。周六实在走不开，下周我来好好看看你的新家。", Why = "先接住他的心意，再改期" },
        new() { Tag = "轻松简短", Text = "恭喜乔迁！周六陪娃复查去不了，下周请你吃饭，好好暖和一下。", Why = "短，但更软" }
    ];

    public static List<ReplyItem> Shorter() =>
    [
        new() { Tag = "轻松简短", Text = "恭喜乔迁！周六陪娃复查去不了，下周请你吃饭", Why = "短，但原因和补偿都在" },
        new() { Tag = "温暖体贴", Text = "太为你高兴了，周六得陪娃复查，下周上门暖房", Why = "先高兴，再把事说清" },
        new() { Tag = "委婉拒绝", Text = "这日子真不想错过，周六走不开，替我和老王说声抱歉", Why = "拒绝清楚，也不扫兴" }
    ];

    public static List<ReplyItem> Polish() =>
    [
        new() { Tag = "谦和", Text = "好久不见呀！我在深圳这边，还在慢慢打拼。你现在在哪儿发展？", Why = "“瞎混”换成“打拼”，更体面" },
        new() { Tag = "温暖", Text = "真的是我😄 好久不见！我在深圳，你呢？最近都还好吧", Why = "先回应她的惊喜，再关心她" },
        new() { Tag = "幽默", Text = "被你抓到了😂 在深圳搬砖呢，你在哪？有空来深圳我请你吃饭", Why = "自嘲拉近距离，还顺手约了饭" }
    ];
}
