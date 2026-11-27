package com.example;

import androidx.annotation.ReturnThis;

interface JChain {
    @ReturnThis
    JChain next();
}

class JChainImpl implements JChain {
    @Override
    public JChain next() {
        return new JChainImpl();
    }
}

class JOverloadBase {
    @ReturnThis
    JOverloadBase copy(int x) {
        return this;
    }

    JOverloadBase copy(String s) {
        return this;
    }
}

class JOverloads extends JOverloadBase {
    @Override
    JOverloadBase copy(int x) {
        return new JOverloadBase();
    }

    @Override
    JOverloadBase copy(String s) {
        return new JOverloadBase();
    }
}
