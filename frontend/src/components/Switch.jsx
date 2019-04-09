import React, { Component } from 'react';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';
import classnames from 'classnames';
import ReactSwitch from 'rc-switch';

import 'components/Switch.scss';

class Switch extends Component {
  constructor(props) {
    super(props);
  }

  @autobind
  onChange(value) {
    if (this.props.onChange) {
      this.props.onChange(value);
    }
  }

  render() {
    return (
      <ReactSwitch
        className={classnames('Switch', this.props.type)}
        onChange={this.onChange}
        disabled={this.props.disabled}
        checkedChildren={this.props.checkedChildren}
        unCheckedChildren={this.props.unCheckedChildren}
        checked={this.props.value}
      />
    );
  }
}

Switch.defaultProps = {
  disabled: false,
  type: 'default',
  checkedChildren: 'ON',
  unCheckedChildren: 'OFF'
};

Switch.propTypes = {
  checkedChildren: PropTypes.node,
  unCheckedChildren: PropTypes.node,
  onChange: PropTypes.func,
  disabled: PropTypes.bool,
  value: PropTypes.bool,
  type: PropTypes.string
};

export default Switch;
