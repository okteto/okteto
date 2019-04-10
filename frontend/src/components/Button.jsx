import React, { Component } from 'react';
import PropTypes from 'prop-types';
import classnames from 'classnames';

import Icon from 'components/Icon';

import colors from 'colors.scss';
import 'components/Button.scss';

const supportedColors = ['red', 'green', 'grey'];

class Button extends Component {
  constructor(props) {
    super(props);
  }

  render() {
    const color = supportedColors.includes(this.props.color) ? this.props.color : 'default';
    const defaultIconColor = this.props.light ? colors.black900 : colors.white800;
    return (
      <button 
        title={this.props.title}
        className={classnames('Button layout horizontal center', this.props.className, color, { 
          solid: this.props.solid,
          light: this.props.light,
          secondary: this.props.secondary,
          frameless: this.props.frameless,
          captionless: !this.props.children,
          'with-icon': !!this.props.icon
        })}
        disabled={this.props.disabled}
        onClick={this.props.onClick}>
        {this.props.icon &&
          <Icon icon={this.props.icon} size={this.props.iconSize} color={defaultIconColor} />
        }
        {this.props.children && 
          <div className="button-content flex-auto">{this.props.children}</div>
        }
      </button>
    );
  }
}

Button.defaultProps = {
  solid: false,
  light: false,
  iconSize: "16"
};

Button.propTypes = {
  title: PropTypes.string,
  icon: PropTypes.string,
  iconSize: PropTypes.string,
  solid: PropTypes.bool,
  light: PropTypes.bool,
  secondary: PropTypes.bool,
  color: PropTypes.string,
  onClick: PropTypes.func,
  className: PropTypes.string,
  children: PropTypes.node,
  frameless: PropTypes.bool,
  disabled: PropTypes.bool
};

export default Button;
